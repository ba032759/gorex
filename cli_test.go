package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
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
	// An empty value is the default, as for every flag (F-101).
	for v, want := range map[string]bool{"": true, "=": true, "=true": true, "=1": true, "=false": false, "=0": false} {
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

// rex keymap exits with 1 when a problem is an error, as rex config
// check does, and with 0 for a warning alone.
//
// spec: F-178, F-179
func TestKeymapErrorsExit1(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	path := filepath.Join(dir, "keymap.json")
	os.WriteFile(path, []byte(`{"preset": "nope"}`), 0o644)
	for _, args := range [][]string{{"keymap"}, {"keymap", "--json"}, {"config", "check"}} {
		if out, code := cli(t, nil, args...); code != exitFailed || !strings.Contains(out, "unknown preset") {
			t.Errorf("%s of a bad file: %d %s", strings.Join(args, " "), code, out)
		}
	}
	os.WriteFile(path, []byte(`{"preset": "linux", "bindings": [{"key": "ctrl+shift+k", "action": "nope.nothing"}]}`), 0o644)
	for _, args := range [][]string{{"keymap"}, {"config", "check"}} {
		if out, code := cli(t, nil, args...); code != 0 || !strings.Contains(out, "warning") {
			t.Errorf("%s with a warning: %d %s", strings.Join(args, " "), code, out)
		}
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
	if _, err := os.Stat(filepath.Join(os.Getenv("GOREX_DIR"), "state.json")); err == nil {
		t.Error("--discard left the saved sessions")
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the idle server did not stop")
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
	if _, err := os.Stat(state); err == nil {
		t.Error("--forget left the saved sessions")
	}
	<-done
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

// rex server logs prints the newest 200 lines, --all the whole log, -n
// another count; --follow ends with the server.
//
// spec: F-430
func TestServerLogs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	var long strings.Builder
	for i := range 250 {
		fmt.Fprintf(&long, "line %d\n", i)
	}
	os.WriteFile(filepath.Join(dir, "server.log"), []byte(long.String()), 0o600)
	if out, code := cli(t, nil, "server", "logs"); code != 0 || strings.Count(out, "\n") != 200 || !strings.HasPrefix(out, "line 50\n") {
		t.Errorf("logs: %d, %d lines from %.8q", code, strings.Count(out, "\n"), out)
	}
	if out, code := cli(t, nil, "server", "logs", "--all"); code != 0 || out != long.String() {
		t.Errorf("logs --all: %d, %d lines", code, strings.Count(out, "\n"))
	}
	if _, code := cli(t, nil, "server", "logs", "--all", "-n", "3"); code != exitUsage {
		t.Errorf("logs --all -n 3: %d", code)
	}
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

// ~NAME in --cwd is that user's home.
//
// spec: F-132
func TestHomeOfUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	t.Setenv("HOME", t.TempDir())
	for p, want := range map[string]string{
		"~":                       os.Getenv("HOME"),
		"~/src":                   filepath.Join(os.Getenv("HOME"), "src"),
		"~" + u.Username + "/src": filepath.Join(u.HomeDir, "src"),
		"~" + u.Username:          u.HomeDir,
		"~no-such-user-gorex/src": filepath.Join(must(os.Getwd()), "~no-such-user-gorex/src"),
		"src":                     filepath.Join(must(os.Getwd()), "src"),
	} {
		o := rex.CreateOptions{Dir: p}
		if fixDir(&o); o.Dir != want {
			t.Errorf("--cwd %s: %s, want %s", p, o.Dir, want)
		}
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// rex window focus takes next and prev in any case.
//
// spec: F-142
func TestWindowFocusNextAnyCase(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Create: &rex.CreateOptions{Dir: "/tmp"}}, &s); err != nil {
		t.Fatal(err)
	}
	here := map[string]string{"REX_SESSION": s["session_id"].(string)}
	cli(t, here, "window", "new")
	cli(t, here, "window", "new")
	for _, step := range []struct {
		word string
		want int
	}{{"1", 0}, {"NEXT", 1}, {"Next", 2}, {"PREV", 1}, {"Prev", 0}, {"next", 1}} {
		if out, code := cli(t, here, "window", "focus", step.word); code != 0 {
			t.Fatalf("window focus %s: %d %s", step.word, code, out)
		}
		m, _ := c.Model()
		ss := m.Sessions[0]
		if got := slices.IndexFunc(ss.Windows, func(w *rex.Window) bool { return w.ID == ss.Active }); got != step.want {
			t.Errorf("window focus %s: window %d, want %d", step.word, got, step.want)
		}
	}
}

// The help and the skill say session, window and block; tab and pane
// only for how the app shows them.
//
// spec: F-103, F-432
func TestHelpTerms(t *testing.T) {
	texts := map[string]string{"overview": overview(), "skill": skillGuide}
	for _, c := range allCommands(rexCommands) {
		texts["rex help "+c.path] = commandHelp(c)
	}
	// A sentence that names a tab or a pane is of the app, unless it
	// names keys, an action or a command, or quotes the user.
	sentence := regexp.MustCompile(`[.;:]\s+|\n\n`)
	word := regexp.MustCompile(`(?i)(^|[^-+.\w])(tabs?|panes?)\b([^.]|\.\s|\.?$)`)
	for name, text := range texts {
		for _, s := range sentence.Split(text, -1) {
			s = strings.Join(strings.Fields(s), " ")
			if word.MatchString(s) && !strings.Contains(s, "app shows") && !strings.Contains(s, "app's") && !strings.Contains(s, "enter, tab") && !strings.Contains(s, `"`) {
				t.Errorf("%s: %s", name, s)
			}
		}
	}
}

// The help's prose says what the code does: the skill's hooks, the
// events printed without -s, the keymap's default preset.
//
// spec: F-169, F-178, F-185
func TestHelpProse(t *testing.T) {
	for _, e := range hookEvents {
		if !strings.Contains(skillHelp, e) {
			t.Errorf("rex help skill does not name the hook of %s", e)
		}
	}
	if !strings.Contains(eventsHelp, "every session's window and block events") {
		t.Error("rex help events does not say what it prints without -s")
	}
	if !strings.Contains(keymapHelp, `"macosKeybindings": true`) {
		t.Error("rex help keymap does not say when macos is the default")
	}
}

// Without GOREX_DIR, the data directory is gorex in the user's
// configuration directory: the directory MyGo gives the app, which the app
// sets as GOREX_DIR (main.go), so that rex outside GoRex finds it too.
//
// spec: F-106
func TestDataDirectory(t *testing.T) {
	config := t.TempDir()
	t.Setenv("GOREX_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", config)
	if d := rex.Dir(); d != filepath.Join(config, "gorex") {
		t.Errorf("data directory %s", d)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/home/someone")
	if d := rex.Dir(); d != "/home/someone/.config/gorex" {
		t.Errorf("data directory %s", d)
	}
}

// silentServer listens on the server's socket of a directory of its own
// and answers nothing, or only hello, with the protocol version hello,
// when hello is not 0.
func silentServer(t *testing.T, hello int) {
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
					if json.Unmarshal(sc.Bytes(), &req) == nil && hello != 0 && req.Op == "hello" {
						data, _ := json.Marshal(rex.Hello{Version: hello})
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
	for _, hello := range []int{0, rex.ProtocolVersion} {
		silentServer(t, hello)
		for _, args := range [][]string{{"session", "ls"}, {"block", "ls"}, {"split"}, {"version"}} {
			start := time.Now()
			_, code := cli(t, nil, append([]string{"--timeout", "200ms"}, args...)...)
			if took := time.Since(start); code == 0 && args[0] != "version" || took > 2*time.Second {
				t.Errorf("hello %d, %v: exit %d after %s", hello, args, code, took)
			}
		}
	}
}

// spec: F-104, F-166, T-103
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

// spec: F-106
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

// spec: F-109, F-118
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

// The wide event of a command names the command as the tree declares it,
// never the text of its arguments.
//
// spec: F-121
func TestWideCommandNames(t *testing.T) {
	for args, want := range map[string]string{
		"send ls":                 "send",
		"send -b x echo secret":   "send",
		"session new work":        "session new",
		"session list":            "session ls",
		"block send-key enter":    "block send-key",
		"session nope secret":     "session",
		"secret words here":       "",
		"server logs -n 3":        "server logs",
		"do pane.split dir=right": "do",
	} {
		if got := commandPath(strings.Fields(args)); got != want {
			t.Errorf("rex %s: command %q, want %q", args, got, want)
		}
	}
	dir := t.TempDir()
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("GOREX_EVENTS", "")
	cli(t, nil, "--autostart=false", "send", "ls", "secret")
	files, _ := filepath.Glob(filepath.Join(dir, "events", "*.jsonl"))
	var b []byte
	for _, f := range files {
		x, _ := os.ReadFile(f)
		b = append(b, x...)
	}
	if !strings.Contains(string(b), `"command":"send"`) || strings.Contains(string(b), "secret") {
		t.Errorf("the events of rex send ls secret: %s", b)
	}
	// A word that looks like a flag, and a usage error's text, stay out.
	cli(t, nil, "--autostart=false", "send", "-b", "x", "-rf-secret")
	cli(t, nil, "--autostart=false", "split", "--secret-flag")
	b = b[:0]
	for _, f := range files {
		x, _ := os.ReadFile(f)
		b = append(b, x...)
	}
	if strings.Contains(string(b), "secret") || !strings.Contains(string(b), `"flags":"--block"`) ||
		!strings.Contains(string(b), `"n_unknown_flags":1`) || !strings.Contains(string(b), `"error":"usage"`) {
		t.Errorf("flag-like words or a usage error's text in the events: %s", b)
	}
}

// --help or -h after a command prints its help, as rex help COMMAND does,
// and exits with 0; after --, it is the command's.
//
// spec: F-103
func TestHelpAfterCommand(t *testing.T) {
	t.Setenv("GOREX_DIR", t.TempDir()) // no server runs there
	for args, topic := range map[string]string{
		"split --help": "split", "split -h": "split", "split --ratio 30 --help": "split",
		"session new -h": "session new", "ls --help": "ls", "server run --help": "server run", "send-keys -h": "send-keys",
	} {
		want, _ := cli(t, nil, "help", topic)
		if out, code := cli(t, nil, append([]string{"--autostart=false"}, strings.Fields(args)...)...); code != 0 || out != want || !strings.Contains(out, "Usage") {
			t.Errorf("rex %s: %d %q", args, code, out)
		}
	}
	if _, code := cli(t, nil, "--autostart=false", "split", "--", "grep", "-h"); code != exitNoServer {
		t.Errorf("rex split -- grep -h: %d, want the split tried", code)
	}
}

// noServer sets GOREX_DIR to a directory of its own, where no server
// runs, and returns the path of the log a server started there writes.
func noServer(t *testing.T) string {
	t.Helper()
	dir := testDir(t)
	t.Cleanup(func() { os.RemoveAll(dir) })
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("REX_SERVER", "")
	return filepath.Join(dir, "server.log")
}

// started tells whether rex started a server for the data directory: the
// server it starts writes its log there.
func started(log string) bool {
	_, err := os.Stat(log)
	return err == nil
}

// The tests' executable is not a server: a server rex starts here never
// answers. That a real one does is checked by hand (F-107).
//
// spec: F-107
func TestAutostart(t *testing.T) {
	log := noServer(t)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--autostart=false", "ls"}, exitNoServer},
		{[]string{"--autostart=false", "split"}, exitNoServer},
		{[]string{"version"}, exitOK},
		{[]string{"server", "status"}, exitNoServer},
		{[]string{"server", "stop"}, exitNoServer},
	} {
		if out, code := cli(t, nil, tc.args...); code != tc.code {
			t.Errorf("%v: %d, want %d: %s", tc.args, code, tc.code, out)
		}
		if started(log) {
			t.Fatalf("%v started a server", tc.args)
		}
	}
	start := time.Now()
	if out, code := cli(t, nil, "--timeout", "500ms", "ls"); code != exitNoServer {
		t.Errorf("ls, the server started not answering: %d %s", code, out)
	}
	if took := time.Since(start); took < 500*time.Millisecond || took > 5*time.Second {
		t.Errorf("rex waited %s for the server, with --timeout 500ms", took)
	}
	// rex started its own executable as a server: it tells so in the log.
	var b []byte
	for deadline := time.Now().Add(3 * time.Second); !strings.Contains(string(b), "-server") && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		b, _ = os.ReadFile(log)
	}
	if !strings.Contains(string(b), "-server") {
		t.Errorf("rex started no server: the log has %q", b)
	}
}

// spec: F-108
func TestServerOfAnotherVersion(t *testing.T) {
	silentServer(t, rex.ProtocolVersion+1)
	out, code := cli(t, nil, "ls")
	if code != exitFailed || !strings.Contains(out, fmt.Sprint(rex.ProtocolVersion)) ||
		!strings.Contains(out, fmt.Sprint(rex.ProtocolVersion+1)) || !strings.Contains(out, "open GoRex") {
		t.Errorf("ls with a server of protocol %d: %d %s", rex.ProtocolVersion+1, code, out)
	}
}

// spec: F-110
func TestSessionOfACommand(t *testing.T) {
	c := startTestServer(t)
	ids, blocks := map[string]string{}, map[string]string{}
	for _, l := range []string{"a", "b", "c", "d"} {
		var s map[string]any
		if err := c.Do("session.new", &rex.Args{Label: l}, &s); err != nil {
			t.Fatal(err)
		}
		ids[s["session_id"].(string)], blocks[l] = l, s["block_id"].(string)
	}
	// in is the session rex block ls lists the blocks of.
	in := func(env map[string]string, args ...string) string {
		t.Helper()
		out, code := cli(t, env, append([]string{"block", "ls", "--json"}, args...)...)
		var r struct {
			Blocks []rex.BlockInfo `json:"blocks"`
		}
		if code != 0 || json.Unmarshal([]byte(out), &r) != nil || len(r.Blocks) == 0 {
			t.Fatalf("block ls %v: %d %s", args, code, out)
		}
		return ids[r.Blocks[0].Session]
	}
	m, _ := c.Model()
	sid := func(l string) string {
		return m.Sessions[slices.IndexFunc(m.Sessions, func(s *rex.Session) bool { return s.Label == l })].ID
	}
	ctx := map[string]string{"REX_SESSION": sid("b"), "REX_BLOCK": blocks["c"]}
	if got := in(ctx, "-s", "a"); got != "a" {
		t.Errorf("-s a: %s", got)
	}
	if got := in(ctx); got != "b" {
		t.Errorf("the context session b, the context block in c: %s", got)
	}
	if got := in(map[string]string{"REX_SESSION": "", "REX_BLOCK": blocks["c"]}); got != "c" {
		t.Errorf("the context block in c: %s", got)
	}
	none := map[string]string{"REX_SESSION": "", "REX_BLOCK": ""}
	if got := in(none); got != "a" {
		t.Errorf("no context, no app: %s, not the first", got)
	}
	view := func(l string) {
		v, err := rex.Connect()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { v.Close() })
		if err := v.View(sid(l)); err != nil {
			t.Fatal(err)
		}
	}
	view("d")
	if got := in(none); got != "d" {
		t.Errorf("an app shows d: %s", got)
	}
	view("c")
	if got := in(none); got != "c" {
		t.Errorf("an app showed c last: %s", got)
	}
	for _, s := range m.Sessions {
		c.Do("session.kill", &rex.Args{Session: s.ID}, nil)
	}
	if out, code := cli(t, none, "block", "ls"); code != exitTarget {
		t.Errorf("block ls with no session: %d %s", code, out)
	}
}

// windowIn is the window rex window inspect shows.
func windowIn(t *testing.T, env map[string]string, args ...string) string {
	t.Helper()
	out, code := cli(t, env, append([]string{"window", "inspect", "--json"}, args...)...)
	var r struct {
		ID string `json:"window_id"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil {
		t.Fatalf("window inspect %v: %d %s", args, code, out)
	}
	return r.ID
}

// spec: F-111
func TestWindowOfACommand(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Label: "w"}, &s); err != nil {
		t.Fatal(err)
	}
	var w2, w3 map[string]any
	c.Do("window.new", &rex.Args{Session: "w"}, &w2)
	c.Do("window.new", &rex.Args{Session: "w"}, &w3) // the active window
	m, _ := c.Model()
	w1 := m.Sessions[0].Windows[0].ID
	env := map[string]string{"REX_SESSION": "", "REX_BLOCK": w2["block_id"].(string)}
	if got := windowIn(t, env); got != w2["window_id"] {
		t.Errorf("no -s, no -w: window %s, not the context block's %s", got, w2["window_id"])
	}
	if got := windowIn(t, env, "-s", "w"); got != w3["window_id"] {
		t.Errorf("-s: window %s, not the active %s", got, w3["window_id"])
	}
	out, code := cli(t, env, "block", "ls", "--json", "-w", w1)
	var r struct {
		Blocks []rex.BlockInfo `json:"blocks"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil || len(r.Blocks) != 1 || r.Blocks[0].Window != w1 {
		t.Errorf("block ls -w %s: %d %s", w1, code, out)
	}
}

// spec: F-115
func TestWhereAWindowMatches(t *testing.T) {
	c := startTestServer(t)
	for _, l := range []string{"a", "b"} {
		if err := c.Do("session.new", &rex.Args{Label: l, WindowLabel: "build"}, nil); err != nil {
			t.Fatal(err)
		}
		c.Do("window.new", &rex.Args{Session: l, WindowLabel: "only-" + l}, nil)
	}
	m, _ := c.Model()
	a, b := m.Sessions[0], m.Sessions[1]
	env := map[string]string{"REX_SESSION": a.ID, "REX_BLOCK": ""}
	if got := windowIn(t, env, "build"); got != a.Windows[0].ID {
		t.Errorf("build: %s, not a's", got)
	}
	if out, code := cli(t, env, "window", "inspect", "only-b"); code != exitTarget {
		t.Errorf("a window of another session by label: %d %s", code, out)
	}
	if got := windowIn(t, env, "-s", "b", "build"); got != b.Windows[0].ID {
		t.Errorf("-s b build: %s, not b's", got)
	}
	for _, args := range [][]string{{b.Windows[1].ID}, {"-s", "a", b.Windows[1].ID}} {
		if got := windowIn(t, env, args...); got != b.Windows[1].ID {
			t.Errorf("%v, the full ID of a window of b: %s", args, got)
		}
	}
}

// sideOf tells where block id is beside anchor in a layout: right, left,
// above or below; "" when they share no split.
func sideOf(l *rex.Layout, anchor, id string) string {
	if l == nil || l.IsLeaf() {
		return ""
	}
	switch {
	case l.A.Block == anchor && l.B.Block == id && l.Vertical:
		return "below"
	case l.A.Block == anchor && l.B.Block == id:
		return "right"
	case l.B.Block == anchor && l.A.Block == id && l.Vertical:
		return "above"
	case l.B.Block == anchor && l.A.Block == id:
		return "left"
	}
	return sideOf(l.A, anchor, id) + sideOf(l.B, anchor, id)
}

// spec: F-122
func TestFirstCommands(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Label: "first", Create: &rex.CreateOptions{Dir: "/tmp"}}, &s); err != nil {
		t.Fatal(err)
	}
	b1 := s["block_id"].(string)
	here := map[string]string{"REX_SESSION": s["session_id"].(string), "REX_BLOCK": b1}
	if out, code := cli(t, here, "list"); code != 0 || !strings.Contains(out, "first") || !strings.Contains(out, "[1]") || !strings.Contains(out, b1) {
		t.Errorf("list: %d %s", code, out)
	}

	// new-tab is window new; two words or more are the command.
	window := func(args ...string) (*rex.Window, []string) {
		t.Helper()
		out, code := cli(t, here, append(args, "--focus=false")...)
		if code != 0 {
			t.Fatalf("%v: %d %s", args, code, out)
		}
		m, _ := c.Model()
		for _, w := range m.Sessions[0].Windows {
			if w.ID == strings.TrimSpace(out) {
				return w, m.Blocks[w.Layout.Block].Command
			}
		}
		t.Fatalf("%v printed %q, no window", args, out)
		return nil, nil
	}
	if w, cmd := window("new-tab", "logs"); w.Label != "logs" || cmd != nil {
		t.Errorf("new-tab logs: label %q, command %q", w.Label, cmd)
	}
	for _, args := range [][]string{{"new-tab", "sleep", "30"}, {"window", "new", "sleep", "30"}} {
		if w, cmd := window(args...); w.Label != "" || !slices.Equal(cmd, []string{"sleep", "30"}) {
			t.Errorf("%v: label %q, command %q", args, w.Label, cmd)
		}
	}

	// send-keys types text, its <CR>, <Esc>, \n, \r and \e as keys.
	for _, args := range [][]string{{"send-keys", b1, `echo one-$((1+1))<CR>`}, {"send-keys", `echo two-$((1+2))\n`}, {"send-keys", `echo three-$((1+3))\r`}} {
		if out, code := cli(t, here, args...); code != 0 {
			t.Errorf("%v: %d %s", args, code, out)
		}
	}
	waitScreen(t, c, b1, "one-2")
	waitScreen(t, c, b1, "two-3")
	waitScreen(t, c, b1, "three-4")
	var cat map[string]any
	c.Do("block.split", &rex.Args{Block: b1, NoFocus: true, Create: &rex.CreateOptions{Command: []string{"cat", "-v"}}}, &cat)
	if out, code := cli(t, here, "send-keys", cat["block_id"].(string), `a<Esc>b\ec<CR>`); code != 0 {
		t.Errorf("send-keys with escapes: %d %s", code, out)
	}
	waitScreen(t, c, cat["block_id"].(string), "a^[b^[c")
	c.Do("block.close", &rex.Args{Block: cat["block_id"].(string)}, nil)

	// split's first flags, and a command without --.
	for _, tc := range []struct {
		args []string
		side string
	}{
		{[]string{"-t", b1}, "right"},
		{[]string{"--target", b1}, "right"},
		{[]string{"-b", b1, "-v"}, "below"},
		{[]string{"-b", b1, "--vertical"}, "below"},
		{[]string{"-b", b1, "--down"}, "below"},
		{[]string{"-b", b1, "--below"}, "below"},
		{[]string{"-b", b1, "--horizontal"}, "right"},
		{[]string{"-b", b1, "--right"}, "right"},
		{[]string{"-b", b1, "--beside"}, "right"},
		{[]string{"-b", b1, "sleep", "30"}, "right"},
	} {
		out, code := cli(t, here, append([]string{"split", "--focus=false"}, tc.args...)...)
		id := strings.TrimSpace(out)
		m, _ := c.Model()
		_, w := m.BlockPlace(b1)
		if code != 0 || w == nil || sideOf(w.Layout, b1, id) != tc.side {
			t.Errorf("split %v: %d %q, want %s of %s", tc.args, code, out, tc.side, b1)
		}
		if tc.args[len(tc.args)-1] == "30" && (m.Blocks[id] == nil || !slices.Equal(m.Blocks[id].Command, []string{"sleep", "30"})) {
			t.Errorf("split %v: not the command", tc.args)
		}
		c.Do("block.close", &rex.Args{Block: id}, nil)
	}
}

// waitScreen waits for a block's screen to show want, and returns it.
func waitScreen(t *testing.T, c *rex.Client, block, want string) string {
	t.Helper()
	var text string
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(30 * time.Millisecond) {
		var out map[string]any
		if c.Do("capture", &rex.Args{Block: block, Trim: true}, &out) == nil {
			if text, _ = out["text"].(string); strings.Contains(text, want) {
				return text
			}
		}
	}
	t.Fatalf("block %s never showed %q; it shows %q", block, want, text)
	return ""
}

// spec: F-123
func TestVersion(t *testing.T) {
	noServer(t)
	want := fmt.Sprintf("rex %s (GoRex, protocol %d)", rex.Version, rex.ProtocolVersion)
	if out, code := cli(t, nil, "version"); code != 0 || !strings.Contains(out, want) || strings.Contains(out, "server") {
		t.Errorf("version without a server: %d %s", code, out)
	}
	startTestServer(t)
	server := fmt.Sprintf("server pid %d, protocol %d", os.Getpid(), rex.ProtocolVersion)
	if out, code := cli(t, nil, "version"); code != 0 || !strings.Contains(out, want) || !strings.Contains(out, server) {
		t.Errorf("version with a server: %d %s", code, out)
	}
}

// spec: F-133
func TestBlockCommandStarts(t *testing.T) {
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	os.Mkdir(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "gorex-on-profile"), []byte("#!/bin/sh\necho on-the-profile-path\n"), 0o755)
	os.WriteFile(filepath.Join(home, ".profile"), []byte(`PATH="`+bin+`:$PATH"
export GOREX_PROFILE=read
alias gorex-alias='echo aliased'
`), 0o644)
	nosh := filepath.Join(home, "nosh") // a shell T-125 does not name
	os.Symlink(testShell.path, nosh)
	t.Setenv("HOME", home)
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Create: &rex.CreateOptions{Dir: "/tmp"}}, &s); err != nil {
		t.Fatal(err)
	}
	b := s["block_id"].(string)
	waitScreen(t, c, b, testShell.prompt)
	c.Do("send", &rex.Args{Block: b, Data: "echo zero=$0 profile=$GOREX_PROFILE\r"}, nil)
	waitScreen(t, c, b, "zero=-sh profile=read")

	run := func(args ...string) string {
		t.Helper()
		out, code := cli(t, nil, append([]string{"run", "--keep-open", "--focus=false"}, args...)...)
		if code != 0 {
			t.Fatalf("run %v: %d %s", args, code, out)
		}
		return strings.TrimSpace(out)
	}
	waitScreen(t, c, run("--", "gorex-on-profile"), "on-the-profile-path")
	if text := waitScreen(t, c, run("--", "gorex-alias"), "exited with code 127"); strings.Contains(text, "aliased") {
		t.Errorf("a command ran as the profile's alias: %q", text)
	}
	waitScreen(t, c, run("--", "printf", "[%s]", "a b", "$HOME", "*", "'"), "[a b][$HOME][*][']")

	blocks := func() int {
		m, _ := c.Model()
		return len(m.Blocks)
	}
	for _, sh := range []string{"none", nosh} {
		waitScreen(t, c, run("--shell", sh, "--", "sh", "-c", "echo direct=[$GOREX_PROFILE]"), "direct=[]")
		n := blocks()
		if out, code := cli(t, nil, "run", "--shell", sh, "--", "gorex-on-profile"); code != exitFailed {
			t.Errorf("--shell %s, a command off the PATH: %d %s", sh, code, out)
		}
		if blocks() != n {
			t.Errorf("--shell %s, a command not found: a block was created", sh)
		}
	}
}

// spec: F-148
func TestDirectionWords(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Label: "d"}, &s); err != nil {
		t.Fatal(err)
	}
	b1 := s["block_id"].(string)
	here := map[string]string{"REX_SESSION": s["session_id"].(string), "REX_BLOCK": b1}
	words := map[string][]string{
		"right": {"right", "r", "beside", "horizontal"},
		"left":  {"left", "l"},
		"above": {"above", "up", "top", "u"},
		"below": {"below", "down", "bottom", "d", "vertical"},
	}
	cases := func(yield func(side, word string) bool) {
		for side, ws := range words {
			for _, w := range ws {
				for _, word := range []string{w, strings.ToUpper(w), strings.ToUpper(w[:1]) + w[1:]} {
					if !yield(side, word) {
						return
					}
				}
			}
		}
	}
	layout := func() *rex.Layout {
		m, _ := c.Model()
		_, w := m.BlockPlace(b1)
		return w.Layout
	}
	for side, word := range cases {
		for _, args := range [][]string{{"split", "--split=" + word}, {"run", "--split", word, "--", "sh"}} {
			out, code := cli(t, here, append(args, "--focus=false")...)
			id := strings.TrimSpace(out)
			if got := sideOf(layout(), b1, id); code != 0 || got != side {
				t.Errorf("%v: %d %q, %q of the block, want %s", args, code, out, got, side)
			}
			c.Do("block.close", &rex.Args{Block: id}, nil)
		}
	}
	// b1 | b2 over b3.
	var sp map[string]any
	c.Do("block.split", &rex.Args{Block: b1, Direction: "right"}, &sp)
	b2 := sp["block_id"].(string)
	c.Do("block.split", &rex.Args{Block: b2, Direction: "below"}, &sp)
	b3 := sp["block_id"].(string)
	for side, word := range cases {
		if out, code := cli(t, here, "move", b3, word, b1); code != 0 || sideOf(layout(), b1, b3) != side {
			t.Errorf("move %s: %d %s, %q of the block", word, code, out, sideOf(layout(), b1, b3))
		}
		cli(t, here, "move", b3, "below", b2)
		l := layout()
		before := map[bool]float32{false: l.Ratio, true: l.B.Ratio}
		if out, code := cli(t, here, "resize", b3, word, "--amount", "1"); code != 0 {
			t.Errorf("resize %s: %d %s", word, code, out)
			continue
		}
		l = layout()
		vertical := side == "above" || side == "below"
		after := map[bool]float32{false: l.Ratio, true: l.B.Ratio}[vertical]
		if grew := after > before[vertical]; grew != (side == "right" || side == "below") || after == before[vertical] {
			t.Errorf("resize %s: ratio %v to %v", word, before[vertical], after)
		}
	}
	for _, args := range [][]string{
		{"split", "--split=diagonal"}, {"run", "--split=diagonal", "--", "sh"},
		{"move", b3, "diagonal", b1}, {"resize", b3, "diagonal"}, {"split", "--split=rightward"},
	} {
		if out, code := cli(t, here, args...); code != exitUsage {
			t.Errorf("%v: %d %s", args, code, out)
		}
	}
}

// spec: F-155
func TestBlockTypes(t *testing.T) {
	noServer(t)
	if out, code := cli(t, nil, "--autostart=false", "block", "types"); code != 0 || strings.Fields(out)[0] != "com.superlogical.terminal.shell" || strings.Count(out, "\n") != 1 {
		t.Errorf("block types: %d %q", code, out)
	}
	if out, code := cli(t, nil, "--autostart=false", "block", "new"); code != exitUsage || !strings.Contains(out, "rex run") {
		t.Errorf("block new: %d %s", code, out)
	}
}

// spec: F-176
func TestDoRunsNoLua(t *testing.T) {
	startTestServer(t)
	for _, args := range [][]string{
		{"-e", "print(1)"}, {"--eval=print(1)"}, {"init.lua"}, {"./tool"}, {"dir/tool"}, {`dir\tool`}, {"-"},
	} {
		if out, code := cli(t, nil, append([]string{"do"}, args...)...); code != exitUsage || !strings.Contains(out, "no Lua") {
			t.Errorf("do %v: %d %s", args, code, out)
		}
	}
	// Rex's flags are taken and ignored: the action goes to the app, of
	// which there is none.
	out, code := cli(t, nil, "do", "--origin", "x", "--for", "1s", "--all", "--fail-fast", "-a", "pane.split", "client.tab.new")
	if code != exitFailed || !strings.Contains(out, "no app is connected") {
		t.Errorf("do with Rex's flags: %d %s", code, out)
	}
}

// spec: F-177
func TestActionsList(t *testing.T) {
	log := noServer(t)
	out, code := cli(t, nil, "actions", "--json")
	var r struct {
		Actions []actionInfo `json:"actions"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &r) != nil || len(r.Actions) < 10 {
		t.Fatalf("actions --json: %d %s", code, out)
	}
	if started(log) {
		t.Error("rex actions started a server")
	}
	if !slices.IsSortedFunc(r.Actions, func(x, y actionInfo) int { return strings.Compare(x.Name, y.Name) }) {
		t.Error("the actions are not sorted by name")
	}
	for _, a := range r.Actions {
		if a.Title == "" {
			t.Errorf("%s has no title", a.Name)
		}
	}
	if i := slices.IndexFunc(r.Actions, func(a actionInfo) bool { return a.Name == "pane.resize" }); i < 0 || !strings.Contains(r.Actions[i].Args, "direction") {
		t.Errorf("pane.resize does not say its arguments: %+v", r.Actions)
	}
	if out, code := cli(t, nil, "actions"); code != 0 || !strings.HasPrefix(out, "NAME") || !strings.Contains(out, "Split Pane") {
		t.Errorf("actions: %d %s", code, out)
	}
}

// spec: F-181
func TestServerStatus(t *testing.T) {
	log := noServer(t)
	if out, code := cli(t, nil, "server", "status"); code != exitNoServer || !strings.HasPrefix(out, "not running") {
		t.Errorf("status without a server: %d %s", code, out)
	}
	if started(log) {
		t.Error("rex server status started a server")
	}
	c := startTestServer(t)
	c.Do("session.new", &rex.Args{}, nil)
	exe, _ := rex.Executable()
	out, code := cli(t, nil, "server", "status")
	for _, want := range []string{fmt.Sprintf("running, pid %d, up ", os.Getpid()), rex.SocketPath(), rex.Dir(), "sessions 1", exe} {
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("status: %d %s, without %q", code, out, want)
		}
	}
}

// That rex server start starts a real server is checked by hand (F-183):
// the tests' executable is none.
//
// spec: F-183, F-107
func TestServerStart(t *testing.T) {
	log := noServer(t)
	if out, code := cli(t, nil, "--autostart=false", "--timeout", "500ms", "server", "start"); code != exitNoServer {
		t.Errorf("start, the server started not answering: %d %s", code, out)
	}
	if !started(log) {
		t.Error("rex --autostart=false server start started no server")
	}

	c := startTestServer(t)
	c.Do("session.new", &rex.Args{Label: "kept"}, nil)
	h, _ := c.Hello()
	if out, code := cli(t, nil, "server", "start"); code != 0 {
		t.Errorf("start with a server: %d %s", code, out)
	}
	if h2, err := c.Hello(); err != nil || h2.PID != h.PID || !h2.Started.Equal(h.Started) {
		t.Errorf("start with a server: %+v %v, was %+v", h2, err, h)
	}
	if m, _ := c.Model(); len(m.Sessions) != 1 {
		t.Errorf("start with a server: %d sessions", len(m.Sessions))
	}
	if out, code := cli(t, nil, "server", "run"); code != exitFailed {
		t.Errorf("run with a server: %d %s", code, out)
	}
}

// spec: F-183
func TestServerRunsInForeground(t *testing.T) {
	noServer(t)
	t.Setenv("SHELL", testShell.path)
	opts = global{autostart: false, timeout: 10 * time.Second}
	done := make(chan int)
	go func() { done <- runCLI([]string{"server", "run"}) }()
	var c *rex.Client
	var err error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if c, err = rex.DialFor(time.Second); err == nil {
			break
		}
	}
	if err != nil {
		t.Fatalf("rex server run: no server answers: %v", err)
	}
	select {
	case code := <-done:
		t.Fatalf("rex server run ended with %d while its server ran", code)
	case <-time.After(300 * time.Millisecond):
	}
	c.ShutdownForget()
	c.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("rex server run ended with %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("rex server run goes on after its server stopped")
	}
}

// spec: F-190
func TestInspect(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Label: "work", WindowLabel: "main", Create: &rex.CreateOptions{Dir: "/tmp"}}, &s); err != nil {
		t.Fatal(err)
	}
	b1 := s["block_id"].(string)
	var sp map[string]any
	c.Do("block.split", &rex.Args{Block: b1, NoFocus: true, Create: &rex.CreateOptions{Label: "ended", Command: []string{"sh", "-c", "exit 3"}, KeepOpen: true}}, &sp)
	b2 := sp["block_id"].(string)
	here := map[string]string{"REX_SESSION": s["session_id"].(string), "REX_BLOCK": b1}
	m, _ := c.Model()
	w := m.Sessions[0].Windows[0]
	contains := func(args []string, wants ...string) {
		t.Helper()
		out, code := cli(t, here, args...)
		for _, want := range wants {
			if code != 0 || !strings.Contains(out, want) {
				t.Errorf("%v: %d %s, without %q", args, code, out, want)
			}
		}
	}
	contains([]string{"session", "inspect", "work"}, "session  "+s["session_id"].(string), "label    work", "created  ", "windows  1",
		"* 1 "+rex.ShortID(w.ID)+"  main  2 blocks")
	contains([]string{"window", "inspect", "main"}, "window   "+w.ID, "label    main", "session  "+s["session_id"].(string)+" (work)",
		"index    1", "focused  "+b1, "blocks", b1, b2+"  ended")

	if out, code := cli(t, here, "status", "-b", b1, "done", "--app", "tests", "-m", "built"); code != 0 {
		t.Fatalf("status: %d %s", code, out)
	}
	if _, code := cli(t, here, "wait", b2); code != 3 {
		t.Fatalf("wait: %d", code)
	}
	ls, _ := cli(t, here, "block", "ls")
	contains([]string{"block", "inspect", b1}, "block", b1, "session", "(work)", "window", w.ID+" (1)", "focused", "true", "pid", "shell", "program",
		"idle", "dir", "/tmp", "size", "output", "attached", "status", "done  tests: built")
	contains([]string{"block", "inspect", "ended"}, "label", "ended", "exited", "status 3")
	if !strings.Contains(ls, "exited (3)") || !strings.Contains(ls, "done") {
		t.Errorf("block ls: %s", ls)
	}
}

// rex server stop returns once the server has ended, so that a start
// right after it starts a new server.
//
// spec: F-182
func TestServerStopWaits(t *testing.T) {
	startTestServer(t)
	if out, code := cli(t, nil, "server", "stop"); code != 0 {
		t.Fatalf("server stop: %d %s", code, out)
	}
	if serverRuns() {
		t.Error("a server still answers after rex server stop returned")
	}
}
