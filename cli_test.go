package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorex/internal/rex"
)

// GoRex runs as rex with a command of rex's, after rex's global options.
//
// spec: F-100
func TestGoRexRunsAsRex(t *testing.T) {
	for args, want := range map[string]bool{
		"ls":                          true,
		"--timeout 5s ls":             true,
		"-C app --autostart=false ls": true,
		"--timeout=5s help":           true,
		"--timeout x ls":              true, // rex says what is wrong
		"-S host ls":                  true,
		"":                            false,
		"--timeout 5s":                false,
		"--nope ls":                   false,
		"--timeout 5s nope":           false,
		"-psn_0_1234":                 false,
	} {
		if got := isCLI(strings.Fields(args)); got != want {
			t.Errorf("GoRex %s: rex %v, want %v", args, got, want)
		}
	}
}

// --autostart is a boolean as every other flag.
//
// spec: F-101
func TestAutostartIsBoolean(t *testing.T) {
	for v, want := range map[string]bool{"": true, "=true": true, "=1": true, "=false": false, "=0": false} {
		var g global
		if _, err := globalFlags(&g).parseLeading([]string{"--autostart" + v, "ls"}); err != nil || g.autostart != want {
			t.Errorf("--autostart%s: %v %v", v, g.autostart, err)
		}
	}
	for _, v := range []string{"yes", "no", "on", "2"} {
		var g global
		if _, err := globalFlags(&g).parseLeading([]string{"--autostart=" + v, "ls"}); exitCode(err) != exitUsage {
			t.Errorf("--autostart=%s: %v", v, err)
		}
	}
}

// capture's formats are the values --format takes: another is a wrong
// command line, before rex connects.
//
// spec: F-164
func TestCaptureFormats(t *testing.T) {
	t.Setenv("GOREX_DIR", t.TempDir()) // no server runs there
	for _, f := range []string{"html", "foo", "TEXT"} {
		if out, code := cli(t, nil, "--autostart=false", "capture", "--format", f); code != exitUsage || !strings.Contains(out, "text or vt") {
			t.Errorf("--format %s: %d %s", f, code, out)
		}
	}
	if _, code := cli(t, nil, "--autostart=false", "capture", "--format", "vt"); code != exitNoServer {
		t.Errorf("--format vt without a server: %d", code)
	}
}

// An action of the app with no app connected fails: rex do and rex
// config reload exit with 1.
//
// spec: F-175, F-180
func TestNoAppFails(t *testing.T) {
	startTestServer(t)
	for _, args := range [][]string{{"do", "client.tab.new"}, {"config", "reload"}, {"config", "reload", "--json"}} {
		if out, code := cli(t, nil, args...); code != exitFailed || !strings.Contains(out, "no app is connected") {
			t.Errorf("%s: %d %s", strings.Join(args, " "), code, out)
		}
	}
}

// rex config check --json prints what it found, and exits with 1 when a
// problem is an error.
//
// spec: F-179
func TestConfigCheckJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	path := filepath.Join(dir, "keymap.json")
	os.WriteFile(path, []byte(`{"preset": "linux", "bindings": [{"key": "ctrl+shift+k", "action": "none"}]}`), 0o644)
	out, code := cli(t, nil, "config", "check", "--json")
	var r configCheck
	if err := json.Unmarshal([]byte(out), &r); err != nil || code != 0 || r.Path != path || !r.Exists || r.Preset != "linux" || r.Problems == nil {
		t.Errorf("config check --json: %d %s %+v", code, out, r)
	}
	os.WriteFile(path, []byte(`{"preset": "nope"}`), 0o644)
	out, code = cli(t, nil, "config", "check", "--json")
	if code != exitFailed || !strings.Contains(out, `"problems": [`) || !strings.Contains(out, "unknown preset") {
		t.Errorf("config check --json of a bad file: %d %s", code, out)
	}
}

// rex client ls -q prints the IDs alone; --short=false, full ones.
//
// spec: F-156
func TestClientLsIDs(t *testing.T) {
	startTestServer(t)
	out, code := cli(t, nil, "client", "ls", "--all", "-q")
	lines := strings.Fields(out)
	if code != 0 || len(lines) == 0 || strings.Contains(out, "KIND") || !strings.HasPrefix(lines[0], "…") {
		t.Errorf("client ls -q: %d %q", code, out)
	}
	out, _ = cli(t, nil, "client", "ls", "--all", "-q", "--short=false")
	if lines = strings.Fields(out); len(lines) == 0 || !strings.HasPrefix(lines[0], "client:") {
		t.Errorf("client ls -q --short=false: %q", out)
	}
}

// serveAlone runs a server that no client of the test holds on to, and
// returns a channel closed when it ends.
func serveAlone(t *testing.T) chan struct{} {
	t.Helper()
	dir := testDir(t)
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("SHELL", testShell.path)
	for _, k := range []string{"REX_SESSION", "REX_BLOCK", "GOREX_SESSION", "REX_CLIENT"} {
		t.Setenv(k, "")
	}
	done := make(chan struct{})
	go func() {
		rex.Serve()
		close(done)
	}()
	for i := 0; i < 200 && !serverRuns(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	return done
}

// rex server stop --if-idle stops a server with no session and no other
// client, and leaves one in use running, with status 1.
//
// spec: F-182
func TestServerStopIfIdle(t *testing.T) {
	done := serveAlone(t)
	out, code := cli(t, nil, "new", "busy", "--", "sleep", "30")
	if code != 0 {
		t.Fatalf("new: %d %s", code, out)
	}
	if out, code = cli(t, nil, "server", "stop", "--if-idle"); code != exitFailed || !strings.Contains(out, "in use") {
		t.Errorf("stop --if-idle with a session: %d %s", code, out)
	}
	if !serverRuns() {
		t.Fatal("the server stopped")
	}
	cli(t, nil, "kill", "busy")
	if out, code = cli(t, nil, "server", "stop", "--if-idle", "--discard"); code != 0 {
		t.Errorf("stop --if-idle when idle: %d %s", code, out)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the idle server did not stop")
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("GOREX_DIR"), "state.json")); err == nil {
		t.Error("--discard left the saved sessions")
	}
}

// rex server stop --discard, and GoRex's --forget, delete the saved
// sessions; with no server, it only deletes them.
//
// spec: F-182
func TestServerStopDiscard(t *testing.T) {
	done := serveAlone(t)
	state := filepath.Join(os.Getenv("GOREX_DIR"), "state.json")
	cli(t, nil, "new", "kept")
	if out, code := cli(t, nil, "server", "stop", "--forget"); code != 0 {
		t.Fatalf("stop --forget: %d %s", code, out)
	}
	<-done
	if _, err := os.Stat(state); err == nil {
		t.Error("--forget left the saved sessions")
	}
	os.WriteFile(state, []byte(`{"version": 1}`), 0o600)
	if out, code := cli(t, nil, "--autostart=false", "server", "stop", "--discard"); code != 0 {
		t.Errorf("stop --discard without a server: %d %s", code, out)
	}
	if _, err := os.Stat(state); err == nil {
		t.Error("--discard without a server left the saved sessions")
	}
	if _, code := cli(t, nil, "--autostart=false", "server", "stop"); code != exitNoServer {
		t.Errorf("stop without a server: %d", code)
	}
}

// rex server logs -n prints the newest lines; --follow ends with the
// server.
//
// spec: F-183
func TestServerLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	os.WriteFile(filepath.Join(dir, "server.log"), []byte("one\ntwo\nthree\n"), 0o600)
	if out, code := cli(t, nil, "server", "logs", "-n", "2"); code != 0 || out != "two\nthree\n" {
		t.Errorf("logs -n 2: %d %q", code, out)
	}
	if out, _ := cli(t, nil, "server", "logs", "--lines=9"); out != "one\ntwo\nthree\n" {
		t.Errorf("logs --lines=9: %q", out)
	}
	if out, code := cli(t, nil, "server", "logs", "--follow"); code != 0 || out != "one\ntwo\nthree\n" {
		t.Errorf("logs --follow without a server: %d %q", code, out)
	}
	if b, at, err := readFrom(filepath.Join(dir, "server.log"), 4); err != nil || string(b) != "two\nthree\n" || at != 14 {
		t.Errorf("readFrom: %q %d %v", b, at, err)
	}
	if _, code := cli(t, nil, "server", "logs", "-n", "0"); code != exitUsage {
		t.Errorf("logs -n 0: %d", code)
	}
}

// An interrupt ends a command with 130, as Rex's.
//
// spec: F-104
func TestInterruptExits130(t *testing.T) {
	if args := os.Getenv("GOREX_TEST_RUN"); args != "" {
		os.Exit(runCLI(strings.Fields(args)))
	}
	serveAlone(t)
	os.WriteFile(filepath.Join(os.Getenv("GOREX_DIR"), "server.log"), []byte("ready\n"), 0o600)
	cmd := exec.Command(os.Args[0], "-test.run=^TestInterruptExits130$")
	cmd.Env = append(os.Environ(), "GOREX_TEST_RUN=server logs --follow")
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, _ := bufio.NewReader(out).ReadString('\n')
	if line != "ready\n" {
		t.Fatalf("logs --follow printed %q", line)
	}
	cmd.Process.Signal(os.Interrupt)
	if err := cmd.Wait(); cmd.ProcessState.ExitCode() != exitInterrupted {
		t.Errorf("interrupted: %v, status %d", err, cmd.ProcessState.ExitCode())
	}
}

// spec: F-124
func TestCompletionPowerShell(t *testing.T) {
	if out, code := cli(t, nil, "completion", "powershell"); code != exitUsage || !strings.Contains(out, "Linux") {
		t.Errorf("completion powershell: %d %s", code, out)
	}
}
