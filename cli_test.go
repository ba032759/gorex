package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"gorex/internal/rex"
)

// silentServer listens on the server's socket of a directory of its own
// and answers nothing, or only hello when hello is set.
func silentServer(t *testing.T, hello bool) {
	t.Helper()
	dir := testDir(t)
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("REX_SERVER", "")
	l, err := net.Listen("unix", rex.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				sc := bufio.NewScanner(conn)
				for sc.Scan() {
					var req rex.Request
					if json.Unmarshal(sc.Bytes(), &req) == nil && hello && req.Op == "hello" {
						data, _ := json.Marshal(rex.Hello{Version: rex.ProtocolVersion})
						b, _ := json.Marshal(rex.Response{ID: req.ID, Data: data})
						conn.Write(append(b, '\n'))
					}
				}
			}()
		}
	}()
}

// spec: F-101
func TestTimeoutLimitsEveryRequest(t *testing.T) {
	for _, hello := range []bool{false, true} {
		silentServer(t, hello)
		for _, args := range [][]string{{"session", "ls"}, {"block", "ls"}, {"split"}, {"version"}} {
			start := time.Now()
			_, code := cli(t, nil, append([]string{"--timeout", "200ms"}, args...)...)
			if took := time.Since(start); code == 0 && args[0] != "version" || took > 2*time.Second {
				t.Errorf("hello %v, %v: exit %d after %s", hello, args, code, took)
			}
		}
	}
}

// spec: F-104, F-168, T-103
func TestServerGoneExits4(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Create: &rex.CreateOptions{Command: []string{"sleep", "30"}}}, &s); err != nil {
		t.Fatal(err)
	}
	block := s["block_id"].(string)
	if _, code := cli(t, nil, "wait", "--for", "200ms", block); code != exitFailed {
		t.Errorf("wait --for: exit %d, not 1", code)
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		c.ShutdownForget()
	}()
	if out, code := cli(t, nil, "wait", block); code != exitNoServer {
		t.Errorf("wait as the server went away: exit %d, not 4: %s", code, out)
	}
}

// spec: F-130, T-108, T-124
func TestJSONKeysInSnakeCase(t *testing.T) {
	c := startTestServer(t)
	if err := c.Do("session.new", &rex.Args{Label: "work"}, nil); err != nil {
		t.Fatal(err)
	}
	out, code := cli(t, nil, "block", "ls", "--json")
	var res struct {
		Blocks []map[string]any `json:"blocks"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &res) != nil || len(res.Blocks) != 1 {
		t.Fatalf("block ls --json: %d %s", code, out)
	}
	for _, k := range []string{"exit_code", "last_output", "last_input"} {
		if _, ok := res.Blocks[0][k]; !ok {
			t.Errorf("block ls --json has no %s: %v", k, res.Blocks[0])
		}
	}
	for k := range res.Blocks[0] {
		if strings.ToLower(k) != k {
			t.Errorf("block ls --json has the key %s", k)
		}
	}
}

// spec: F-140, F-113
func TestSeveralTargetsAllOrNothing(t *testing.T) {
	c := startTestServer(t)
	for _, l := range []string{"a", "b", "c"} {
		if err := c.Do("session.new", &rex.Args{Label: l}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var s map[string]any
	c.Do("session.resolve", &rex.Args{Session: "a"}, &s)
	env := map[string]string{"REX_SESSION": s["session_id"].(string)}
	if _, code := cli(t, env, "window", "new", "--focus=false"); code != 0 { // a second window in a
		t.Fatalf("window new: exit %d", code)
	}
	m, _ := c.Model()
	w2 := m.Sessions[0].Windows[1].ID
	w1 := m.Sessions[0].Windows[0].ID
	b1 := m.Sessions[0].Windows[0].Layout.Block
	n := func() (sessions, windows int) {
		m, _ := c.Model()
		return len(m.Sessions), len(m.Sessions[0].Windows)
	}
	for _, args := range [][]string{
		{"kill", "a", "nope", "b"},
		{"window", "close", w1, "nope"},
		{"close", b1, "nope"},
	} {
		if _, code := cli(t, env, args...); code != exitTarget {
			t.Errorf("%v: exit %d, not 3", args, code)
		}
		if ss, ws := n(); ss != 3 || ws != 2 {
			t.Errorf("%v changed something: %d sessions, %d windows in a", args, ss, ws)
		}
	}
	// A target named twice acts once.
	if _, code := cli(t, env, "window", "close", w2, w2); code != 0 {
		t.Errorf("window close twice: exit %d", code)
	}
	if _, code := cli(t, env, "kill", "b", "c", "b"); code != 0 {
		t.Errorf("kill: exit %d", code)
	}
	if ss, ws := n(); ss != 1 || ws != 1 {
		t.Errorf("after closing: %d sessions, %d windows", ss, ws)
	}
}

// spec: F-106, F-120
func TestRexServerBeforeGorexDir(t *testing.T) {
	c := startTestServer(t)
	if err := c.Do("session.new", &rex.Args{Label: "here"}, nil); err != nil {
		t.Fatal(err)
	}
	sock := rex.SocketPath()
	other := testDir(t)
	t.Cleanup(func() { os.RemoveAll(other) })
	t.Setenv("GOREX_DIR", other)
	t.Setenv("REX_SERVER", "unix:"+sock)
	if out, code := cli(t, nil, "ls"); code != 0 || !strings.Contains(out, "here") {
		t.Errorf("rex ls by REX_SERVER: %d %s", code, out)
	}
	t.Setenv("REX_SERVER", "unix:"+other+"/nobody.sock")
	if _, code := cli(t, nil, "--timeout", "300ms", "ls"); code != exitNoServer {
		t.Errorf("REX_SERVER of no server: exit %d, not 4", code)
	}
	if _, err := os.Stat(other + "/server.log"); err == nil {
		t.Error("rex started a server that would not listen on REX_SERVER")
	}
	t.Setenv("REX_SERVER", "host:7000")
	if _, code := cli(t, nil, "ls"); code != exitUsage {
		t.Errorf("REX_SERVER of a remote server: exit %d, not 2", code)
	}
}

// spec: F-109, F-118, F-120
func TestNoGorexSession(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Create: &rex.CreateOptions{Command: []string{"env"}, KeepOpen: true}}, &s); err != nil {
		t.Fatal(err)
	}
	first := s["block_id"].(string)
	var text string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !strings.Contains(text, "REX_BLOCK="); time.Sleep(30 * time.Millisecond) {
		var out map[string]any
		if c.Do("capture", &rex.Args{Block: first}, &out) == nil {
			text, _ = out["text"].(string)
		}
	}
	if !strings.Contains(text, "REX_BLOCK="+first) || !strings.Contains(text, "REX_SERVER=unix:") {
		t.Fatalf("the block's environment: %q", text)
	}
	if strings.Contains(text, "GOREX_SESSION") {
		t.Errorf("the block has GOREX_SESSION: %q", text)
	}
	// rex takes no context from GOREX_SESSION: zoom acts on the focused
	// block, not on the one GOREX_SESSION names.
	var sp map[string]any
	if err := c.Do("block.split", &rex.Args{Block: first, NoFocus: true}, &sp); err != nil {
		t.Fatal(err)
	}
	if _, code := cli(t, map[string]string{"GOREX_SESSION": sp["block_id"].(string)}, "zoom"); code != 0 {
		t.Fatalf("zoom: exit %d", code)
	}
	if m, _ := c.Model(); m.Sessions[0].Windows[0].Zoom != first {
		t.Errorf("zoom acted on %q, not the focused block %s", m.Sessions[0].Windows[0].Zoom, first)
	}
}
