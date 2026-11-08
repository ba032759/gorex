package rex

import (
	"bytes"
	"encoding/base64"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// The Program Status Protocol (OSC 7501), revision 0.4, as Rex has it: a
// program tells its terminal what it does, idle, working, done, blocked on
// the user or failed, and why, by a sequence it prints:
//
//	ESC ] 7501 ; state=blocked:kind=permission:app=claude-code:msg=<base64> ESC \
//
// A terminal keeps a record per id, each report replacing its record
// whole. The server keeps the records of every block, which clients show.
// https://www.superlogical.com/rex/docs/build/program-status

// StatusRecord is what a program last reported for an id.
type StatusRecord struct {
	// ID is the record's path, "" for the root record.
	ID    string `json:"id"`
	State string `json:"state"`
	// Kind is what a blocked record waits for: permission, question or
	// auth; "" when it did not say.
	Kind string `json:"kind,omitempty"`
	// Progress is a percentage of a working or blocked record, -1 when it
	// gave none.
	Progress int       `json:"progress"`
	App      string    `json:"app,omitempty"`
	Title    string    `json:"title,omitempty"`
	Msg      string    `json:"msg,omitempty"`
	Updated  time.Time `json:"updated"`
}

// The states, and the kinds of a blocked record.
var (
	statusStates = []string{"idle", "working", "done", "blocked", "error"}
	statusKinds  = []string{"permission", "question", "auth"}
)

// The protocol's limits.
const (
	statusMaxSeq     = 4096
	statusMaxKey     = 16
	statusMaxMsg     = 2732 // encoded
	statusMaxMsgDec  = 2048
	statusMaxTitle   = 256
	statusMaxTitleD  = 192
	statusMaxApp     = 32
	statusMaxID      = 128
	statusMaxSegment = 32
	statusMaxDepth   = 8
	statusMaxRecords = 256
)

// statusQueryReply is the answer to "OSC 7501 ; ?": the protocol is
// supported, with every state and kind of its revision.
const statusQueryReply = "\x1b]7501;?\x1b\\"

var errBadReport = errors.New("not a valid status report")

// parseStatus parses the body of a report, after "7501;". A query is
// "?". A malformed pair is skipped; a report breaking a limit, with bad
// base64 or control characters in its text, is refused whole.
func parseStatus(body string) (rec StatusRecord, query bool, err error) {
	if strings.TrimSpace(body) == "?" {
		return rec, true, nil
	}
	if len(body) > statusMaxSeq {
		return rec, false, errBadReport
	}
	rec.Progress = -1
	pairs := map[string]string{}
	for _, p := range strings.Split(body, ":") {
		k, v, ok := strings.Cut(p, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if !ok || k == "" || len(k) > statusMaxKey || !isLower(k) || !validValue(v) {
			continue
		}
		pairs[k] = v // the last one wins
	}
	rec.State = pairs["state"]
	if rec.State != "clear" && !slices.Contains(statusStates, rec.State) {
		return rec, false, errBadReport
	}
	if id, ok := pairs["id"]; ok {
		if !validStatusID(id) {
			return rec, false, errBadReport // never fall back to the root
		}
		rec.ID = id
	}
	if k := pairs["kind"]; rec.State == "blocked" && slices.Contains(statusKinds, k) {
		rec.Kind = k
	}
	if p, ok := pairs["progress"]; ok && (rec.State == "working" || rec.State == "blocked") {
		if n, err := strconv.Atoi(p); err == nil && n >= 0 && n <= 100 {
			rec.Progress = n
		}
	}
	if a := pairs["app"]; a != "" && len(a) <= statusMaxApp && validApp(a) {
		rec.App = a
	}
	if rec.Title, err = decodeText(pairs["title"], statusMaxTitle, statusMaxTitleD); err != nil {
		return rec, false, err
	}
	if rec.Msg, err = decodeText(pairs["msg"], statusMaxMsg, statusMaxMsgDec); err != nil {
		return rec, false, err
	}
	return rec, false, nil
}

func isLower(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

// validValue is the protocol's set of a value's bytes.
func validValue(v string) bool {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("_.,+/=-", c) >= 0) {
			return false
		}
	}
	return true
}

func validApp(a string) bool {
	for i := 0; i < len(a); i++ {
		c := a[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("_.+-", c) >= 0) {
			return false
		}
	}
	return true
}

func validStatusID(id string) bool {
	if id == "" || len(id) > statusMaxID {
		return false
	}
	segs := strings.Split(id, "/")
	if len(segs) > statusMaxDepth {
		return false
	}
	for _, s := range segs {
		if s == "" || len(s) > statusMaxSegment || !validApp(s) {
			return false
		}
	}
	return true
}

// decodeText decodes a base64 text, its padding optional, refusing one
// too long or with control characters, and dropping the invisible
// characters that format text, as direction overrides.
func decodeText(v string, maxEnc, maxDec int) (string, error) {
	if v == "" {
		return "", nil
	}
	if len(v) > maxEnc {
		return "", errBadReport
	}
	b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(v, "="))
	if err != nil || len(b) > maxDec || !utf8.Valid(b) {
		return "", errBadReport
	}
	var out strings.Builder
	for _, r := range string(b) {
		switch {
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
			return "", errBadReport
		case unicode.Is(unicode.Cf, r):
			continue // formatting: direction overrides, zero widths
		}
		out.WriteRune(r)
	}
	return out.String(), nil
}

// statusSet is the records of a block.
type statusSet map[string]*StatusRecord

// apply takes a report, and returns the record it changed, or the ids it
// removed.
func (s statusSet) apply(rec StatusRecord, now time.Time) (changed *StatusRecord, removed []string) {
	if rec.State == "clear" {
		for id := range s {
			if rec.ID == "" || id == rec.ID || strings.HasPrefix(id, rec.ID+"/") {
				removed = append(removed, id)
				delete(s, id)
			}
		}
		sort.Strings(removed)
		return nil, removed
	}
	if _, ok := s[rec.ID]; !ok && len(s) >= statusMaxRecords {
		// The record updated least recently makes room.
		oldest := ""
		for id, r := range s {
			if oldest == "" || r.Updated.Before(s[oldest].Updated) {
				oldest = id
			}
		}
		delete(s, oldest)
		removed = append(removed, oldest)
	}
	rec.Updated = now
	r := rec
	s[rec.ID] = &r
	return &r, removed
}

// drop removes the records of the given states, as the program's process
// ends or a shell prompt begins, and returns their ids.
func (s statusSet) drop(states ...string) []string {
	var removed []string
	for id, r := range s {
		if slices.Contains(states, r.State) {
			removed = append(removed, id)
			delete(s, id)
		}
	}
	sort.Strings(removed)
	return removed
}

// list returns the records, the root first, each with the app of its
// nearest ancestor when it gave none.
func (s statusSet) list() []StatusRecord {
	out := make([]StatusRecord, 0, len(s))
	for _, r := range s {
		c := *r
		for p := c.ID; c.App == "" && p != ""; {
			if i := strings.LastIndexByte(p, '/'); i >= 0 {
				p = p[:i]
			} else {
				p = ""
			}
			if a, ok := s[p]; ok && a.App != "" {
				c.App = a.App
			}
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// StatusSummary picks the record that says most of a block: one blocked
// on the user, then failed, working, done, idle; the latest of a state.
func StatusSummary(rs []StatusRecord) *StatusRecord {
	rank := map[string]int{"blocked": 5, "error": 4, "working": 3, "done": 2, "idle": 1}
	var best *StatusRecord
	for i := range rs {
		r := &rs[i]
		if best == nil || rank[r.State] > rank[best.State] ||
			rank[r.State] == rank[best.State] && r.Updated.After(best.Updated) {
			best = r
		}
	}
	return best
}

// oscEvent is a sequence of the output that the server acts on.
type oscEvent struct {
	kind string // "status", "prompt" (OSC 133 A), "command" (OSC 133 C), "reset" (RIS)
	body string
}

// oscScanner finds the sequences of the output that the server acts on,
// across the reads that split them.
type oscScanner struct {
	carry []byte
}

var (
	oscStart = []byte("\x1b]")
	risSeq   = []byte("\x1bc")
)

// scan returns the events in data, in order.
func (sc *oscScanner) scan(data []byte) []oscEvent {
	var evs []oscEvent
	s := data
	if len(sc.carry) > 0 {
		s = append(sc.carry, data...)
		sc.carry = nil
	}
	i := 0
	for {
		j := bytes.Index(s[i:], oscStart)
		// A full reset before the next OSC.
		if k := bytes.Index(s[i:], risSeq); k >= 0 && (j < 0 || k < j) {
			evs = append(evs, oscEvent{kind: "reset"})
			i += k + len(risSeq)
			continue
		}
		if j < 0 {
			if n := len(s); n > i && s[n-1] == 0x1b {
				sc.carry = []byte{0x1b} // the start of a sequence
			}
			return evs
		}
		start := i + j
		body, end := oscBody(s[start+2:])
		if end < 0 {
			if len(s)-start <= statusMaxSeq && interesting(s[start+2:]) {
				sc.carry = append([]byte(nil), s[start:]...)
			}
			return evs
		}
		switch b := string(body); {
		case strings.HasPrefix(b, "7501;"):
			if end+2 <= statusMaxSeq {
				evs = append(evs, oscEvent{kind: "status", body: b[5:]})
			}
		case b == "133;A" || strings.HasPrefix(b, "133;A;"):
			evs = append(evs, oscEvent{kind: "prompt"})
		case b == "133;C" || strings.HasPrefix(b, "133;C;"):
			evs = append(evs, oscEvent{kind: "command"})
		}
		i = start + 2 + end
	}
}

// oscBody returns the body of an OSC, whose start is cut off, and the
// length through its terminator, BEL or ESC \; -1 when it does not end in
// s.
func oscBody(s []byte) ([]byte, int) {
	for k := 0; k < len(s); k++ {
		switch s[k] {
		case 0x07:
			return s[:k], k + 1
		case 0x1b:
			if k+1 < len(s) {
				if s[k+1] == '\\' {
					return s[:k], k + 2
				}
				return s[:k], k // another sequence: this one ends
			}
			return nil, -1
		}
	}
	return nil, -1
}

// interesting tells an unfinished OSC that may be one the server acts on.
func interesting(b []byte) bool {
	for _, p := range []string{"7501;", "133;"} {
		n := min(len(b), len(p))
		if string(b[:n]) == p[:n] {
			return true
		}
	}
	return false
}
