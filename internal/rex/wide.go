package rex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Wide events: one flat JSON line per unit of work (a request the server
// answers, an event it emits, a rex command that ran), with every field
// that may help explain it later, appended to events/YYYY-MM-DD.jsonl in
// Dir. They are for asking questions nobody planned, with DuckDB:
//
//	duckdb -c "select op, count(*), avg(duration_ms) from read_json_auto('~/.config/GoRex/events/*.jsonl') group by op"
//
// GOREX_EVENTS=off turns them off. Files older than eventsKeep are removed.
// The fields are described in .claude/skills/gorex-events/SKILL.md.

// W is a wide event under construction.
type W map[string]any

const eventsKeep = 30 * 24 * time.Hour

var wide struct {
	once    sync.Once
	ch      chan []byte
	off     bool
	dropped atomic.Int64
	wg      sync.WaitGroup
}

// EventsDir is where the wide events go.
func EventsDir() string { return filepath.Join(Dir(), "events") }

func wideStart() {
	wide.once.Do(func() {
		if v := os.Getenv("GOREX_EVENTS"); v == "off" || v == "0" {
			wide.off = true
			return
		}
		wide.ch = make(chan []byte, 4096)
		wide.wg.Add(1)
		go wideWriter()
	})
}

// Wide records an event, adding the fields every event has. It never
// blocks: when the writer falls behind, events are dropped, and counted
// in the next one that gets through.
func Wide(kind string, w W) {
	wideStart()
	if wide.off {
		return
	}
	now := time.Now()
	w["ts"] = now.UTC().Format("2006-01-02T15:04:05.000000Z")
	w["kind"] = kind
	w["version"] = Version
	w["protocol"] = ProtocolVersion
	w["pid"] = os.Getpid()
	w["os"] = runtime.GOOS
	w["arch"] = runtime.GOARCH
	if n := wide.dropped.Swap(0); n > 0 {
		w["dropped_before"] = n
	}
	b, err := json.Marshal(w)
	if err != nil {
		return
	}
	select {
	case wide.ch <- append(b, '\n'):
	default:
		wide.dropped.Add(1)
	}
}

// WideFlush waits for the events recorded so far to be written, as a
// command ends. It gives up after a moment.
func WideFlush() {
	if wide.ch == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		for len(wide.ch) > 0 {
			time.Sleep(time.Millisecond)
		}
		// The last line may still be in the writer's hands.
		time.Sleep(2 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
	}
}

func wideWriter() {
	defer wide.wg.Done()
	var (
		f    *os.File
		path string
	)
	for b := range wide.ch {
		// The file of the day, in the directory of the moment: tests move it.
		p := filepath.Join(EventsDir(), time.Now().UTC().Format("2006-01-02")+".jsonl")
		if f == nil || p != path {
			if f != nil {
				f.Close()
			}
			f, path = wideOpen(p), p
		}
		if f != nil {
			// One write per line: O_APPEND keeps the lines of the server
			// and of the commands whole in the one file.
			f.Write(b)
		}
	}
}

func wideOpen(path string) *os.File {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	go widePrune(dir)
	return f
}

func widePrune(dir string) {
	es, _ := os.ReadDir(dir)
	for _, e := range es {
		name := e.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		t, err := time.Parse("2006-01-02", strings.TrimSuffix(name, ".jsonl"))
		if err == nil && time.Since(t) > eventsKeep {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// setNonZero sets the fields whose values are not zero, to keep the lines
// short.
func (w W) setNonZero(kv ...any) W {
	for i := 0; i+1 < len(kv); i += 2 {
		switch v := kv[i+1].(type) {
		case string:
			if v == "" {
				continue
			}
		case int:
			if v == 0 {
				continue
			}
		case bool:
			if !v {
				continue
			}
		case float32:
			if v == 0 {
				continue
			}
		case nil:
			continue
		}
		w[kv[i].(string)] = kv[i+1]
	}
	return w
}
