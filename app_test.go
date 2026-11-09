package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

// testShell is the shell of the tests: its prompt ends with prompt, and
// typing run runs program for a while in dir.
var testShell = func() (s struct{ path, prompt, run, program, dir string }) {
	if runtime.GOOS == "windows" {
		s.path, s.prompt, s.program, s.dir = "cmd.exe", ">", "PING", os.Getenv("SystemRoot")
		s.run = `cd /d ` + s.dir + ` && ping -n 4 127.0.0.1 >nul`
		return
	}
	s.path, s.prompt, s.run, s.program, s.dir = "/bin/sh", "$", "cd /usr/bin && sleep 3", "sleep", "/usr/bin"
	return
}()

// testDir is a directory of the tests' own: in /tmp, whose path is short
// enough for the socket, on macOS.
func testDir(t *testing.T) string {
	base := "/tmp"
	if runtime.GOOS == "windows" {
		base = ""
	}
	dir, err := os.MkdirTemp(base, "gorex")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// startTestServer starts a session server of its own, in a directory of
// its own.
func startTestServer(t *testing.T) *rex.Client {
	t.Helper()
	dir := testDir(t)
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("SHELL", testShell.path)
	t.Setenv("ENV", "")
	for _, k := range []string{"REX_SESSION", "REX_BLOCK", "GOREX_SESSION", "REX_CLIENT"} {
		t.Setenv(k, "")
	}
	done := make(chan struct{})
	go func() {
		rex.Serve()
		close(done)
	}()
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(rex.SocketPath()); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	client, err := rex.Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.ShutdownForget()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		os.RemoveAll(dir)
	})
	return client
}

// newTestApp starts a server and an app on it, whose view runs without a
// window, showing a session of one tab.
func newTestApp(t *testing.T) (*App, *ui.Tester) {
	t.Helper()
	client := startTestServer(t)
	registerFonts()
	prefs = settings{}
	keys = loadKeymap(filepath.Join(rex.Dir(), "keymap.json"))
	a := &App{}
	if err := a.connect(client); err != nil {
		t.Fatal(err)
	}
	a.act("session.new", rex.Args{Create: &rex.CreateOptions{Dir: os.TempDir()}})
	tt := ui.NewTester(a.view, 1000, 620)
	tt.Frame()
	return a, tt
}

// refresh asks the server what runs in the blocks, as the window does
// every half second.
func refresh(a *App) {
	infos, _ := a.client.List()
	byID := map[string]rex.BlockInfo{}
	for _, in := range infos {
		byID[in.ID] = in
	}
	a.apply(byID)
}

// waitFor runs frames until cond holds, or fails after a few seconds.
func waitFor(t *testing.T, tt *ui.Tester, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s; texts %q", what, tt.Texts())
		}
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
}

func shellReady(t *testing.T, tt *ui.Tester, p *Pane) {
	t.Helper()
	waitFor(t, tt, "the shell", func() bool { return p.term != nil && strings.Contains(p.term.Text(), testShell.prompt) })
}

func TestPanesAndTabs(t *testing.T) {
	a, tt := newTestApp(t)
	tab := a.tab()
	p := tab.Focus
	shellReady(t, tt, p)

	// The command palette splits the pane.
	a.openPalette()
	tt.Frame()
	tt.Type("split right")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n := len(tab.panes()); n != 2 {
		t.Fatalf("%d panes after Split Right", n)
	}
	if a.paletteOpen {
		t.Error("the palette stays open")
	}
	right := tab.Focus
	if right == p {
		t.Fatal("the new pane does not have the focus")
	}

	// The header's buttons split down and zoom.
	if err := tt.Click("Split Down"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if n := len(tab.panes()); n != 3 || !tab.Root.B.Vertical {
		t.Fatalf("%d panes after Split Down", n)
	}
	if err := tt.Click("Zoom"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tab.Zoom == nil {
		t.Fatal("not zoomed")
	}
	a.toggleZoom()
	if tab.Zoom != nil {
		t.Fatal("still zoomed")
	}

	// Focus moves between panes by where they are.
	a.moveFocus(-1, 0)
	if tab.Focus != p {
		t.Errorf("focus left went to pane %d, not %d", tab.Focus.ID, p.ID)
	}
	a.moveFocus(1, 0)
	if tab.Focus == p {
		t.Error("focus right stayed")
	}

	// Typing reaches the shell, and the header follows its directory.
	a.setFocus(p)
	a.focusReq = p
	tt.Frame()
	tt.Type(testShell.run)
	tt.Key(0, ui.KeyEnter)
	waitFor(t, tt, testShell.program+" in the header", func() bool {
		refresh(a)
		name, detail := p.label()
		return strings.EqualFold(name, testShell.program) && samePath(detail, shortDir(testShell.dir))
	})

	// A second tab, renamed, then closed with its pane.
	a.newTab(os.TempDir())
	if len(a.tabs) != 2 || a.active != 1 {
		t.Fatalf("%d tabs, active %d", len(a.tabs), a.active)
	}
	a.startRename()
	tt.Frame()
	tt.Type("logs")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Frame()
	if a.tabs[1].Name != "logs" || a.model.Sessions[0].Windows[1].Label != "logs" {
		t.Errorf("tab named %q", a.tabs[1].Name)
	}
	a.closeFocused()
	if len(a.tabs) != 1 || a.active != 0 {
		t.Fatalf("%d tabs, active %d, after closing the last pane of a tab", len(a.tabs), a.active)
	}

	// A shell that exits closes its pane: the server closes the block,
	// and the model says so.
	a.setFocus(right)
	right.term.Send([]byte("exit\r"))
	waitFor(t, tt, "the pane to close", func() bool { return len(a.tab().panes()) == 2 })
}

// TestMirror checks that the window follows what another client does,
// and that a second app finds everything as it was.
func TestMirror(t *testing.T) {
	a, tt := newTestApp(t)
	a.split(false)
	tab := a.tab()
	a.setRatio(tab.Root, 0.3, true)
	a.newTab(testShell.dir)
	a.renameTab(a.tabs[1], "second")
	time.Sleep(100 * time.Millisecond)

	// The CLI's client splits the second tab and zooms it.
	other, err := rex.Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	var res map[string]any
	if err := other.Do("block.split", &rex.Args{Window: a.tabs[1].WID, Direction: "below", Create: &rex.CreateOptions{Label: "logs"}}, &res); err != nil {
		t.Fatal(err)
	}
	if err := other.Do("block.zoom", &rex.Args{Block: "logs", Zoom: "on"}, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the window to follow", func() bool {
		t2 := a.tabs[1]
		return len(t2.panes()) == 2 && t2.Zoom != nil && t2.Zoom.meta.Label == "logs" && a.active == 1
	})
	if name, _ := a.tab().Zoom.label(); name != "logs" {
		t.Errorf("the zoomed pane shows %q", name)
	}

	// Another app, as the next launch, finds them all.
	c2, err := rex.Connect()
	if err != nil {
		t.Fatal(err)
	}
	b := &App{}
	if err := b.connect(c2); err != nil {
		t.Fatal(err)
	}
	if len(b.tabs) != 2 || b.tabs[1].Name != "second" || b.active != 1 {
		t.Fatalf("found %d tabs, active %d", len(b.tabs), b.active)
	}
	if r := b.tabs[0].Root; r.Pane != nil || r.Ratio < 0.29 || r.Ratio > 0.31 {
		t.Errorf("first tab's root %+v", r)
	}
	for i, tb := range b.tabs {
		for j, p := range tb.panes() {
			if p.SID != a.tabs[i].panes()[j].SID {
				t.Errorf("pane %d of tab %d is another block", j, i)
			}
		}
	}
}

func TestSessions(t *testing.T) {
	a, tt := newTestApp(t)
	first := a.session
	a.newSession()
	if a.session == first || len(a.model.Sessions) != 2 || len(a.tabs) != 1 {
		t.Fatalf("session %s of %d, %d tabs", a.session, len(a.model.Sessions), len(a.tabs))
	}
	tt.Frame()
	if _, ok := tt.Find("Session Session 2"); !ok {
		t.Errorf("no session chip; texts %q", tt.Texts())
	}
	// The switcher lists the sessions, the next one first.
	a.openSessions()
	tt.Frame()
	items := a.paletteItems()
	if len(items) != 3 || items[2].title != "New Session" {
		t.Fatalf("switcher %+v", items)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.session != first {
		t.Errorf("Enter in the switcher shows %s, not %s", a.session, first)
	}
	a.cycleSession(1)
	second := a.session
	a.openPrompt("Rename Session", "", "", func(s string) {
		a.act("session.rename", rex.Args{Session: second, Label: s})
	})
	tt.Frame()
	tt.Type("work")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if ss := a.currentSession(); ss.Label != "work" {
		t.Errorf("session label %q", ss.Label)
	}
	// A session that ends shows another.
	a.closeSession()
	if a.session != first || len(a.model.Sessions) != 1 {
		t.Errorf("after closing: session %s of %d", a.session, len(a.model.Sessions))
	}
	if err := a.runAction("session.select", map[string]any{"session": "1"}); err != nil {
		t.Error(err)
	}
}

func TestLabels(t *testing.T) {
	home, _ := os.UserHomeDir()
	site, short := filepath.Join(home, "Sites", "rex-snake"), filepath.Join("~", "Sites", "rex-snake")
	p := &Pane{info: rex.BlockInfo{Shell: "zsh", Program: "zsh", Idle: true, Dir: site}}
	if n, d := p.label(); n != "zsh" || d != short {
		t.Errorf("shell label %q %q", n, d)
	}
	p.info = rex.BlockInfo{Shell: "fish", Program: "lazygit", Dir: site}
	p.title = "lazygit ~/Sites/rex-snake"
	if n, d := p.label(); n != "Git Changes" || d != short {
		t.Errorf("lazygit label %q %q", n, d)
	}
	p.info.Program = "codex"
	p.title = "Stress-test Snake demo | rex-snake"
	if n, d := p.label(); n != p.title || d != "" {
		t.Errorf("codex label %q %q", n, d)
	}
	p.info, p.title = rex.BlockInfo{Shell: "zsh", Program: "ssh", Args: []string{"ssh", "-p", "22", "box.local"}}, ""
	if n, d := p.label(); n != "SSH" || d != "box.local" {
		t.Errorf("ssh label %q %q", n, d)
	}
	// Windows's consoles name the shell in their title as a program runs.
	p.info, p.title = rex.BlockInfo{Shell: "cmd", Program: "node", Dir: site}, `C:\WINDOWS\system32\cmd.exe - node app.js`
	if n, d := p.label(); n != "Node" || d != short {
		t.Errorf("node label %q %q", n, d)
	}
	// A label comes first, with the program beside it.
	p.info, p.title, p.meta.Label = rex.BlockInfo{Shell: "zsh", Program: "node", Dir: site}, "", "api"
	if n, d := p.label(); n != "api" || d != "Node" {
		t.Errorf("labeled label %q %q", n, d)
	}
	if d := shortDir(filepath.FromSlash("/private/tmp/a/b/c/d/e")); d != filepath.FromSlash("…/d/e") {
		t.Errorf("short dir %q", d)
	}
}

// TestCloseButtons presses and releases the close buttons that show while
// the pointer is over a tab or a pane, with a frame between: pressing one
// must not hide it.
func TestCloseButtons(t *testing.T) {
	a, tt := newTestApp(t)
	first := a.tabs[0]
	a.newTab(os.TempDir())
	tt.Frame()
	track, ok := tt.Find("Tabs")
	if !ok {
		t.Fatal("no tab bar")
	}
	// The pointer over the first tab shows its close button.
	tt.Move(track.X+40, track.Y+track.H/2)
	tt.Frame()
	x, ok := tt.Find("Close Tab")
	if !ok {
		t.Fatalf("no close button over the tab; texts %q", tt.Texts())
	}
	tt.Press(x.X+x.W/2, x.Y+x.H/2)
	tt.Frame()
	tt.Release(x.X+x.W/2, x.Y+x.H/2)
	tt.Frame()
	tt.Frame()
	if len(a.tabs) != 1 || slices.Contains(a.tabs, first) {
		t.Fatalf("%d tabs after clicking the first one's close button", len(a.tabs))
	}

	// A pane without the focus shows its buttons under the pointer.
	a.split(false)
	tt.Frame()
	tab := a.tab()
	left := tab.panes()[0]
	if tab.Focus == left {
		t.Fatal("the left pane has the focus")
	}
	b := left.bounds
	tt.Move(b.X+60, b.Y+headerH/2)
	tt.Frame()
	cx, cy := b.X+b.W-8-13, b.Y+headerH/2 // the last button of the header
	tt.Press(cx, cy)
	tt.Frame()
	tt.Release(cx, cy)
	tt.Frame()
	tt.Frame()
	if ps := a.tab().panes(); len(ps) != 1 || ps[0] == left {
		t.Fatalf("%d panes after clicking the left pane's close button", len(ps))
	}
}

// TestPaletteScrolls checks that the wheel scrolls a palette longer than
// it shows, and that the keys bring the selection back into view.
func TestPaletteScrolls(t *testing.T) {
	registerFonts()
	a := &App{}
	tab := &Tab{ID: 1}
	for i := range 20 {
		n := &Node{Pane: &Pane{ID: i, Tab: tab, info: rex.BlockInfo{Shell: fmt.Sprintf("shell%02d", i)}}}
		n.Pane.Node = n
		if tab.Root == nil {
			tab.Root = n
		} else {
			tab.Root = &Node{A: tab.Root, B: n, Ratio: 0.5}
		}
	}
	a.tabs = []*Tab{tab}
	tt := ui.NewTester(func(c *ui.Context) { a.palette(c, colorsOf(c)) }, 1000, 620)
	a.openPalette()
	tt.Frame()
	// A row out of sight finds an empty box.
	top, _ := tt.Find("shell00")
	row10, _ := tt.Find("shell10")
	tt.Move(top.X, top.Y+100)
	tt.Scroll(top.X, top.Y+100, 0, 300)
	tt.Frame()
	if r, _ := tt.Find("shell10"); r.Y != row10.Y-300 {
		t.Errorf("scrolled 300: a row moved from %v to %v", row10.Y, r.Y)
	}
	if r, _ := tt.Find("shell00"); r.H != 0 {
		t.Errorf("scrolled 300: the first row still shows, at %v", r.Y)
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if r, _ := tt.Find("shell01"); r.H == 0 {
		t.Error("the keys chose a row out of sight")
	}
}

func TestMacOSKeybindings(t *testing.T) {
	a, tt := newTestApp(t)
	on := true
	prefs.MacOSKeybindings = &on
	keys = loadKeymap(keymapPath())
	defer func() { prefs.MacOSKeybindings = nil }()
	shellReady(t, tt, a.tab().Focus)
	tt.Frame() // a frame asks for the keymap's keys

	// Cmd+T (emitted as Ctrl+T) opens a new tab.
	tt.Key(ui.Ctrl, ui.KeyT)
	tt.Frame()
	if len(a.tabs) != 2 {
		t.Fatalf("expected 2 tabs after Ctrl+T, got %d", len(a.tabs))
	}

	// Cmd+1 (emitted as Ctrl+1) switches back to first tab.
	tt.Key(ui.Ctrl, ui.Key1)
	tt.Frame()
	if a.active != 0 {
		t.Fatalf("expected tab 0 active after Ctrl+1, got %d", a.active)
	}

	// Cmd+D (emitted as Ctrl+D, or Super+D) splits right.
	tt.Key(ui.Super, ui.KeyD)
	tt.Frame()
	tab0 := a.tabs[0]
	if len(tab0.panes()) != 2 {
		t.Fatalf("expected 2 panes after Super+D, got %d", len(tab0.panes()))
	}

	// Shift+Cmd+D (emitted as Ctrl+Shift+D) splits down.
	tt.Key(ui.Ctrl|ui.Shift, ui.KeyD)
	tt.Frame()
	if len(a.tabs[0].panes()) != 3 {
		t.Fatalf("expected 3 panes after Ctrl+Shift+D, got %d", len(a.tabs[0].panes()))
	}

	// Cmd+W (emitted as Ctrl+W) closes the focused pane.
	tt.Key(ui.Ctrl, ui.KeyW)
	tt.Frame()
	if len(a.tabs[0].panes()) != 2 {
		t.Fatalf("expected 2 panes after Ctrl+W, got %d", len(a.tabs[0].panes()))
	}
}

// spec: T-122
func TestKeymap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keymap.json")
	os.WriteFile(path, []byte(`{"preset": "linux", "bindings": [
		{"key": "ctrl+b>c", "action": "client.tab.new"},
		{"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
		{"key": "ctrl+shift+t", "action": "none"},
		{"key": "Shift+Ctrl+D", "action": "pane.zoom"},
		{"key": "ctrl+x", "action": "no.such.action"},
		{"key": "hyper+x", "action": "pane.zoom"}
	]}`), 0o644)
	km := loadKeymap(path)
	find := func(spec string) *keyBinding {
		seq, err := parseKeySeq(spec)
		if err != nil {
			t.Fatal(err)
		}
		for i, b := range km.bindings {
			if slices.Equal(b.seq, seq) {
				return &km.bindings[i]
			}
		}
		return nil
	}
	if b := find("ctrl+b>c"); b == nil || b.action != "client.tab.new" {
		t.Errorf("sequence %+v", b)
	}
	if b := find("ctrl+b>shift+5"); b == nil || b.args["direction"] != "right" {
		t.Errorf("%% is shift+5: %+v", b)
	}
	if b := find("ctrl+shift+t"); b != nil {
		t.Errorf("unbound key still bound to %s", b.action)
	}
	if b := find("ctrl+shift+d"); b == nil || b.action != "pane.zoom" {
		t.Errorf("rebound key %+v", b)
	}
	if len(km.problems) != 2 || !strings.HasPrefix(km.problems[0], "warning") || !strings.HasPrefix(km.problems[1], "error") {
		t.Errorf("problems %q", km.problems)
	}
	if k := km.keysOf("pane.zoom"); k == "" {
		t.Error("no keys shown for pane.zoom")
	}
}

func TestKeySequence(t *testing.T) {
	a, tt := newTestApp(t)
	os.WriteFile(keymapPath(), []byte(`{"bindings": [{"key": "ctrl+b>c", "action": "client.tab.new"}]}`), 0o644)
	a.reloadKeymap()
	shellReady(t, tt, a.tab().Focus)
	// A "c" alone goes to the terminal.
	tt.Key(0, ui.KeyC)
	tt.Frame()
	if len(a.tabs) != 1 {
		t.Fatal("c alone opened a tab")
	}
	tt.Key(ui.Ctrl, ui.KeyB)
	tt.Frame()
	if len(a.keySeq) != 1 {
		t.Fatal("ctrl+b began no sequence")
	}
	tt.Key(0, ui.KeyC)
	tt.Frame()
	tt.Frame()
	if len(a.tabs) != 2 || len(a.keySeq) != 0 {
		t.Fatalf("%d tabs after ctrl+b c; sequence %v", len(a.tabs), a.keySeq)
	}
	// Escape gives up a sequence.
	tt.Key(ui.Ctrl, ui.KeyB)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if len(a.keySeq) != 0 {
		t.Error("escape left the sequence")
	}
}

// cli runs rex in the test, as the terminal of block would, and returns
// what it printed and its status.
func cli(t *testing.T, env map[string]string, args ...string) (string, int) {
	t.Helper()
	for k, v := range env {
		t.Setenv(k, v)
	}
	r, w, _ := os.Pipe()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	opts = global{autostart: true, timeout: 10 * time.Second}
	code := runCLI(args)
	os.Stdout, os.Stderr = stdout, stderr
	w.Close()
	var b bytes.Buffer
	io.Copy(&b, r)
	return b.String(), code
}

// spec: F-102, F-103, F-104, F-105, F-112, F-116, F-117, F-130, F-131, F-132, F-135, F-138, F-141, F-143, F-144, F-152, F-154, F-160, F-161, F-164, F-166, F-175, T-124
func TestCLI(t *testing.T) {
	c := startTestServer(t)
	var s map[string]any
	if err := c.Do("session.new", &rex.Args{Label: "work", Create: &rex.CreateOptions{Dir: "/tmp"}}, &s); err != nil {
		t.Fatal(err)
	}
	here := map[string]string{"REX_SESSION": s["session_id"].(string), "REX_BLOCK": s["block_id"].(string)}

	out, code := cli(t, here, "split", "--split=below", "--ratio", "30", "--label", "logs", "--focus=false", "--json", "--", "sh")
	if code != 0 {
		t.Fatalf("split: %d %s", code, out)
	}
	var res map[string]any
	if err := json.Unmarshal([]byte(out), &res); err != nil || len(res["block_ids"].([]any)) != 1 {
		t.Fatalf("split --json printed %q", out)
	}
	m, _ := c.Model()
	w := m.Sessions[0].Windows[0]
	if w.Focus != here["REX_BLOCK"] || !w.Layout.Vertical || m.Blocks[res["block_id"].(string)].Label != "logs" {
		t.Errorf("after split: focus %s, layout %+v", w.Focus, w.Layout)
	}

	if out, code = cli(t, here, "send", "-b", "logs", "echo cli-$((6*7))"); code != 0 {
		t.Fatalf("send: %d %s", code, out)
	}
	cli(t, here, "send-key", "-b", "logs", "enter")
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out, "cli-42") && time.Now().Before(deadline) {
		out, _ = cli(t, here, "capture", "-b", "logs", "--trim")
		time.Sleep(30 * time.Millisecond)
	}
	if !strings.Contains(out, "cli-42") {
		t.Errorf("capture %q", out)
	}

	// --wait exits with the command's status; wait does for a block.
	if out, code = cli(t, here, "run", "--wait", "--", "sh", "-c", "exit 7"); code != 7 {
		t.Errorf("run --wait: %d %s", code, out)
	}
	out, _ = cli(t, here, "split", "--focus=false", "--", "sh", "-c", "sleep 0.3; exit 4")
	if _, code = cli(t, here, "wait", strings.TrimSpace(out)); code != 4 {
		t.Errorf("wait: %d", code)
	}

	// Targets and usage.
	if _, code = cli(t, here, "zoom", "-b", "nope"); code != exitTarget {
		t.Errorf("an unknown block: %d", code)
	}
	if _, code = cli(t, here, "split", "--nope"); code != exitUsage {
		t.Errorf("an unknown flag: %d", code)
	}
	if out, code = cli(t, here, "zoom", "-b", "logs", "--on"); code != 0 {
		t.Errorf("zoom: %d %s", code, out)
	}
	if out, code = cli(t, here, "focus", "logs"); code != 0 {
		t.Errorf("focus: %d %s", code, out)
	}
	if out, _ = cli(t, here, "block", "ls"); !strings.Contains(out, "logs") || !strings.Contains(out, "@") {
		t.Errorf("block ls %q", out)
	}
	if out, _ = cli(t, here, "ls"); !strings.Contains(out, "* ") || !strings.Contains(out, "work") {
		t.Errorf("ls %q", out)
	}
	if out, code = cli(t, here, "new", "play", "--json"); code != 0 {
		t.Fatalf("new: %d %s", code, out)
	}
	if out, code = cli(t, here, "session", "rename", "play", "fun"); code != 0 {
		t.Errorf("session rename: %d %s", code, out)
	}
	if out, _ = cli(t, here, "session", "ls", "-q"); len(strings.Fields(out)) != 2 {
		t.Errorf("session ls -q %q", out)
	}
	if out, code = cli(t, here, "kill", "fun"); code != 0 {
		t.Errorf("kill: %d %s", code, out)
	}
	// rex do needs an app.
	if _, code = cli(t, here, "do", "client.tab.new"); code != exitFailed {
		t.Errorf("do without an app: %d", code)
	}
	if out, _ = cli(t, here, "help"); !strings.Contains(out, "rex drives GoRex") {
		t.Errorf("help %q", out)
	}
}

// spec: F-185, F-186, F-187, F-188
func TestSkillSetup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // no agent's program found but by its directory
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("CODEX_HOME", filepath.Join(home, ".codex"))
	os.MkdirAll(filepath.Join(home, ".claude"), 0o755)
	settings := `{
  "model": "opus",
  "permissions": {"allow": ["Bash(ls:*)"]},
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "say done"}]}]},
  "zeta": 1
}`
	path := filepath.Join(home, ".claude", "settings.json")
	os.WriteFile(path, []byte(settings), 0o644)

	r, err := skillSetup(skillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 2 || r.Items[0].State != "created" || r.Items[1].State != "created" {
		t.Fatalf("setup %+v", r.Items)
	}
	b, _ := os.ReadFile(path)
	s := string(b)
	if !strings.Contains(s, hookCommand) || !strings.Contains(s, "say done") || !strings.Contains(s, `"$REX_BIN_DIR/rex\" skill hook claude UserPromptSubmit`) {
		t.Fatalf("settings %s", s)
	}
	// The keys keep their order.
	if i, j, k := strings.Index(s, `"model"`), strings.Index(s, `"permissions"`), strings.Index(s, `"zeta"`); !(i < j && j < k) {
		t.Errorf("keys reordered: %s", s)
	}
	if r, _ = skillSetup(skillOptions{}); r.Items[0].State != "unchanged" || r.Items[1].State != "unchanged" {
		t.Errorf("setup again %+v", r.Items)
	}
	if r, _ = skillStatus(skillOptions{agents: []string{"claude"}}); r.Items[0].State != "current" || r.Items[1].State != "current" {
		t.Errorf("status %+v", r.Items)
	}
	if _, err := skillRemove(skillOptions{agents: []string{"claude"}}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), hookCommand) || !strings.Contains(string(b), "say done") {
		t.Errorf("after remove %s", b)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "rex")); err == nil {
		t.Error("the skill is still there")
	}
	// A skill of the same name rex did not install stays.
	os.MkdirAll(filepath.Join(home, ".claude", "skills", "rex"), 0o755)
	os.WriteFile(filepath.Join(home, ".claude", "skills", "rex", "SKILL.md"), []byte("mine"), 0o644)
	if r, _ = skillSetup(skillOptions{agents: []string{"claude"}}); r.Items[0].State != "foreign" {
		t.Errorf("foreign skill %+v", r.Items)
	}
}

func TestProgramStatusInView(t *testing.T) {
	a, tt := newTestApp(t)
	tab := a.tab()
	first := tab.Focus
	a.split(false) // the focus goes to the new pane; first is out of focus
	tt.Frame()
	report := func(p *Pane, body string) {
		t.Helper()
		if err := a.client.Do("status.report", &rex.Args{Block: p.SID, Data: body}, nil); err != nil {
			t.Fatal(err)
		}
		refresh(a)
		tt.Frame()
	}
	report(first, statusReport("blocked", "", "permission", "", "claude-code", "", "Allow Bash(rm -rf build)?"))
	st := first.programStatus()
	if st == nil || st.State != "blocked" || !first.attention {
		t.Fatalf("status %+v attention %v", st, first.attention)
	}
	if !slices.ContainsFunc(tt.Texts(), func(s string) bool { return s == "Needs permission" }) {
		t.Errorf("no chip in the header; texts %q", tt.Texts())
	}
	if _, detail := tab.label(); detail != "needs permission" {
		t.Errorf("tab detail %q", detail)
	}
	// Back to work, then done: a result, until the user looks at it.
	report(first, statusReport("working", "", "", "40", "claude-code", "", ""))
	if st := first.programStatus(); st == nil || st.State != "working" || st.Progress != 40 {
		t.Errorf("working %+v", st)
	}
	report(first, statusReport("done", "", "", "", "claude-code", "", "All tests pass"))
	if st := first.programStatus(); st == nil || st.State != "done" {
		t.Fatalf("done %+v", st)
	}
	a.focusedWin = true
	a.setFocus(first)
	a.focusReq = first
	tt.Frame()
	tt.Frame()
	if st := first.programStatus(); st != nil {
		t.Errorf("a seen result still shows: %+v", st)
	}
}

// spec: F-189
func TestHookStatus(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "t.jsonl")
	os.WriteFile(tr, []byte(`{"type":"user","message":{"content":"hi"}}
{"type":"assistant","message":{"content":[{"type":"text","text":"First."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use"},{"type":"text","text":"Fixed the   build\nand ran tests."}]}}
`), 0o644)
	for _, c := range []struct {
		event string
		in    hookInput
		want  string
	}{
		{"UserPromptSubmit", hookInput{Prompt: "fix the build"}, "state=working:app=claude-code:msg=" + b64s("fix the build")},
		{"Notification", hookInput{Message: "Claude needs your permission to use Bash", NotificationType: "permission_prompt"}, "state=blocked:kind=permission"},
		{"Notification", hookInput{Message: "Claude is waiting for your input", NotificationType: "idle_prompt"}, ""},
		{"PermissionRequest", hookInput{ToolName: "Bash"}, "state=blocked:kind=permission"},
		{"Stop", hookInput{TranscriptPath: tr}, "state=done:app=claude-code:msg=" + b64s("Fixed the build and ran tests.")},
		{"Stop", hookInput{LastMessage: "Done here"}, "msg=" + b64s("Done here")},
		{"SessionEnd", hookInput{}, "state=clear"},
	} {
		got := hookStatus(c.event, c.in)
		if c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s %+v: %q, want %q", c.event, c.in, got, c.want)
		}
	}
}

func b64s(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
