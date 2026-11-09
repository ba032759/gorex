package rex

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Every request the server answers is a wide event, with how it ended,
// who asked (down to the process), and the size of the server.
//
// spec: T-121
func TestWideEvents(t *testing.T) {
	c, dir := startServer(t)
	act(t, c, "session.new", Args{Label: "work"})
	var out map[string]any
	if err := c.Do("block.focus", &Args{Block: "nope"}, &out); err == nil {
		t.Fatal("focusing no block succeeded")
	}
	WideFlush()
	var lines []map[string]any
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		lines = readWide(t, dir)
		if find(lines, "request", "block.focus") != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	nw := find(lines, "request", "session.new")
	if nw == nil {
		t.Fatalf("no session.new request in %v", lines)
	}
	if nw["ok"] != true || nw["arg_label"] != "work" || nw["client_pid"] != float64(os.Getpid()) {
		t.Errorf("session.new = %v", nw)
	}
	if nw["n_sessions"] != float64(1) || nw["version"] != Version {
		t.Errorf("session.new = %v", nw)
	}
	f := find(lines, "request", "block.focus")
	if f == nil || f["ok"] != false || f["code"] != "not_found" || f["error"] == nil {
		t.Errorf("block.focus = %v", f)
	}
	if find(lines, "event", "session_created") == nil {
		t.Error("no session_created event")
	}
	for _, l := range lines {
		if _, err := time.Parse("2006-01-02T15:04:05.000000Z", l["ts"].(string)); err != nil {
			t.Errorf("ts %v: %v", l["ts"], err)
		}
	}
}

func readWide(t *testing.T, dir string) []map[string]any {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(dir, "events", "*.jsonl"))
	var out []map[string]any
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var m map[string]any
			if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
				t.Errorf("not JSON: %s", sc.Text())
			}
			out = append(out, m)
		}
		f.Close()
	}
	return out
}

func find(lines []map[string]any, kind, what string) map[string]any {
	for _, l := range lines {
		if l["kind"] == kind && (l["op"] == what || l["type"] == what) {
			return l
		}
	}
	return nil
}
