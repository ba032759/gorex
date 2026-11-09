package rex

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// spec: T-110
func TestParseStatus(t *testing.T) {
	r, q, err := parseStatus("state=blocked:kind=permission:app=terraform:progress=30:msg=" + b64("Apply 3?"))
	if err != nil || q || r.State != "blocked" || r.Kind != "permission" || r.App != "terraform" || r.Progress != 30 || r.Msg != "Apply 3?" {
		t.Fatalf("%+v %v %v", r, q, err)
	}
	if _, q, _ := parseStatus("?"); !q {
		t.Error("query not seen")
	}
	// Unknown keys and malformed pairs are skipped; the last of a key wins.
	r, _, err = parseStatus("state=idle:bogus=1:nokey:state=working:progress=101:kind=permission")
	if err != nil || r.State != "working" || r.Progress != -1 || r.Kind != "" {
		t.Errorf("%+v %v", r, err)
	}
	for _, bad := range []string{
		"state=sleeping",                      // unknown state
		"app=x",                               // no state
		"state=done:id=a//b",                  // bad id
		"state=done:msg=" + b64("two\nlines"), // control character
		"state=done:msg=!!!",                  // not base64 (and not a value)
		"state=done:title=" + b64(strings.Repeat("x", 200)), // too long
	} {
		if _, _, err := parseStatus(bad); err == nil && !strings.Contains(bad, "!!!") {
			t.Errorf("accepted %q", bad)
		}
	}
	// Padding is optional; direction overrides are dropped.
	if r, _, err := parseStatus("state=done:msg=" + strings.TrimRight(b64("ok\u202e!"), "=")); err != nil || r.Msg != "ok!" {
		t.Errorf("%+v %v", r, err)
	}
}

func TestStatusSet(t *testing.T) {
	s := statusSet{}
	now := time.Now()
	s.apply(StatusRecord{State: "working", App: "deploy", Progress: -1}, now)
	s.apply(StatusRecord{ID: "us/east", State: "blocked", Kind: "permission", Progress: -1}, now.Add(time.Second))
	s.apply(StatusRecord{ID: "us", State: "done", Progress: -1}, now.Add(2*time.Second))
	l := s.list()
	if len(l) != 3 || l[2].ID != "us/east" || l[2].App != "deploy" {
		t.Fatalf("list %+v", l)
	}
	if sum := StatusSummary(l); sum.State != "blocked" {
		t.Errorf("summary %+v", sum)
	}
	if _, removed := s.apply(StatusRecord{ID: "us", State: "clear"}, now); len(removed) != 2 {
		t.Errorf("clear us removed %v", removed)
	}
	if gone := s.drop("working", "blocked"); len(gone) != 1 || len(s) != 0 {
		t.Errorf("drop %v left %v", gone, s)
	}
}

func TestOSCScanner(t *testing.T) {
	var sc oscScanner
	full := "a\x1b]0;title\x07b\x1b]7501;state=working:app=x\x1b\\c\x1b]133;A\x07\x1bc"
	var evs []oscEvent
	// Fed a byte at a time, as reads may split it anywhere.
	for i := 0; i < len(full); i++ {
		evs = append(evs, sc.scan([]byte{full[i]})...)
	}
	if len(evs) != 3 || evs[0].kind != "status" || evs[0].body != "state=working:app=x" || evs[1].kind != "prompt" || evs[2].kind != "reset" {
		t.Fatalf("events %+v", evs)
	}
}

// spec: F-172, F-173, F-460, F-461, T-110
func TestProgramStatus(t *testing.T) {
	c, _ := startServer(t)
	s := act(t, c, "session.new", Args{Create: &CreateOptions{Dir: "/tmp"}})
	b := s["block_id"].(string)
	waitText(t, c, b, "$")
	status := func() []StatusRecord {
		var out struct {
			Records []StatusRecord `json:"records"`
		}
		if err := c.Do("status.list", &Args{Block: b}, &out); err != nil {
			t.Fatal(err)
		}
		return out.Records
	}
	// A program prints a report and a query; the server answers the query
	// as the terminal.
	act(t, c, "send", Args{Block: b, Data: `printf '\033]7501;state=blocked:kind=question:app=sh:msg=` + b64("Continue?") +
		`\033\\'; sleep 2; printf 'after\n'` + "\r"})
	deadline := time.Now().Add(3 * time.Second)
	for len(status()) == 0 && time.Now().Before(deadline) {
		time.Sleep(30 * time.Millisecond)
	}
	r := status()
	if len(r) != 1 || r[0].State != "blocked" || r[0].Kind != "question" || r[0].Msg != "Continue?" {
		t.Fatalf("records %+v", r)
	}
	// Back at the prompt, blocked is over.
	waitText(t, c, b, "after")
	deadline = time.Now().Add(4 * time.Second)
	for len(status()) != 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if r := status(); len(r) != 0 {
		t.Errorf("blocked outlived the program: %+v", r)
	}
	// A hook reports through the server; done survives the prompt.
	act(t, c, "status.report", Args{Block: b, Data: "state=done:app=claude-code:msg=" + b64("All set")})
	time.Sleep(1500 * time.Millisecond)
	if r := status(); len(r) != 1 || r[0].State != "done" || r[0].App != "claude-code" {
		t.Errorf("records %+v", r)
	}
	if err := c.Do("status.report", &Args{Block: b, Data: "state=nope"}, nil); err == nil {
		t.Error("a bad report was taken")
	}
	// The query's answer reaches the program.
	act(t, c, "send", Args{Block: b, Data: `stty raw -echo; printf '\033]7501;?\033\\'; dd bs=1 count=10 2>/dev/null | od -An -c | tr -d ' \n'; stty sane; echo` + "\r"})
	waitText(t, c, b, "033]7501;?033")
}
