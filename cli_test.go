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
