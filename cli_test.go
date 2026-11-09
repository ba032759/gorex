package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
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
