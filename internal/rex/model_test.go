package rex

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// startServer runs a server of its own in a temporary directory, with
// /bin/sh as the shell, and connects to it.
func startServer(t *testing.T) (*Client, string) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "rex")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOREX_DIR", dir)
	t.Setenv("REX_SERVER", "") // not the server of a GoRex the tests run in
	t.Setenv("SHELL", "/bin/sh")
	t.Setenv("ENV", "") // no interactive startup file for sh
	done := make(chan struct{})
	go func() {
		Serve()
		close(done)
	}()
	for i := 0; i < 200; i++ {
		if _, err := os.Stat(filepath.Join(dir, "server.sock")); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	c, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.ShutdownForget()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		os.RemoveAll(dir)
	})
	return c, dir
}

func act(t *testing.T, c *Client, op string, a Args) map[string]any {
	t.Helper()
	var out map[string]any
	if err := c.Do(op, &a, &out); err != nil {
		t.Fatalf("%s: %v", op, err)
	}
	return out
}

func model(t *testing.T, c *Client) *Model {
	t.Helper()
	m, err := c.Model()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// waitText waits for a block's screen to show want.
func waitText(t *testing.T, c *Client, block, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var text string
	for time.Now().Before(deadline) {
		var out map[string]any
		if err := c.Do("capture", &Args{Block: block, Trim: true}, &out); err == nil {
			text = out["text"].(string)
			if strings.Contains(text, want) {
				return text
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("block %s never showed %q; it shows %q", block, want, text)
	return ""
}

func TestLayoutTree(t *testing.T) {
	l := &Layout{Block: "a"}
	l.insert("a", "b", Right, 0.25, "s1")
	if l.Vertical || l.A.Block != "a" || l.B.Block != "b" || l.Ratio != 0.75 {
		t.Fatalf("split right: %+v", l)
	}
	l.insert("a", "c", Above, 0.5, "s2")
	if got := l.Blocks(); !slices.Equal(got, []string{"c", "a", "b"}) {
		t.Fatalf("blocks %v", got)
	}
	r := l.Rects()
	if a, c := r["a"], r["c"]; a.Y != 0.5 || c.Y != 0 || a.W != 0.75 {
		t.Fatalf("rects %+v", r)
	}
	order := l.Blocks()
	if n := Neighbor(r, order, "a", 0, -1, false); n != "c" {
		t.Errorf("above a: %q", n)
	}
	if n := Neighbor(r, order, "a", 1, 0, false); n != "b" {
		t.Errorf("right of a: %q", n)
	}
	if n := Neighbor(r, order, "b", 1, 0, false); n != "" {
		t.Errorf("right of b: %q", n)
	}
	if n := Neighbor(r, order, "b", 1, 0, true); n == "" || n == "b" {
		t.Errorf("right of b, wrapping: %q", n)
	}
	l = l.remove("a")
	if got := l.Blocks(); !slices.Equal(got, []string{"c", "b"}) {
		t.Fatalf("after remove %v", got)
	}
	if !l.resize("c", Right, 0.1) {
		t.Fatal("no divider for c")
	}
	l.balance()
	if l.Ratio != 0.5 {
		t.Errorf("balanced ratio %v", l.Ratio)
	}
	if l = l.remove("c").remove("b"); l != nil {
		t.Errorf("left %+v", l)
	}
}

// spec: F-113
func TestMatch(t *testing.T) {
	items := []labeled{{id: "session:0000aaaa1111", label: "work"}, {id: "session:0000bbbb1111", label: "Work"}, {id: "session:0000cccc2222"}}
	if _, err := match("session", "work", items, true); err == nil {
		t.Error("two labels matched without ambiguity")
	} else if te := (*TargetError)(nil); !errors.As(err, &te) || len(te.Matches) != 2 {
		t.Errorf("ambiguity %v", err)
	}
	if i, err := match("session", "3", items, true); err != nil || i != 2 {
		t.Errorf("position: %d %v", i, err)
	}
	if i, err := match("session", "2222", items, true); err != nil || i != 2 {
		t.Errorf("suffix: %d %v", i, err)
	}
	if _, err := match("session", "1111", items, true); err == nil {
		t.Error("ambiguous suffix matched")
	}
	if _, err := match("session", "nope", items, true); err == nil {
		t.Error("unknown matched")
	}
}

// spec: F-162, T-160
func TestEncodeKey(t *testing.T) {
	for _, c := range []struct {
		key  string
		app  bool
		want string
	}{
		{"enter", false, "\r"}, {"ctrl+c", false, "\x03"}, {"Ctrl+Shift+A", false, "\x01"},
		{"up", false, "\x1b[A"}, {"up", true, "\x1bOA"}, {"shift+left", false, "\x1b[1;2D"},
		{"alt+x", false, "\x1bx"}, {"shift+tab", false, "\x1b[Z"}, {"f5", false, "\x1b[15~"},
		{"ctrl+[KeyQ]", false, "\x11"}, {"shift+1", false, "!"}, {"escape", false, "\x1b"},
		{"ctrl++", false, ""}, {"pagedown", false, "\x1b[6~"}, {"ctrl+delete", false, "\x1b[3;5~"},
	} {
		got, err := EncodeKey(c.key, c.app)
		if c.want == "" {
			if err == nil {
				t.Errorf("%s: no error", c.key)
			}
			continue
		}
		if err != nil || string(got) != c.want {
			t.Errorf("%s: %q %v, want %q", c.key, got, err, c.want)
		}
	}
}

// spec: F-114, F-132, F-135, F-140, F-142, F-143, F-144, F-145, F-146, F-149, F-150, T-103, T-105
func TestSessionsWindowsBlocks(t *testing.T) {
	c, _ := startServer(t)
	s1 := act(t, c, "session.new", Args{Label: "work", Create: &CreateOptions{Dir: "/tmp"}})
	sid, b1 := s1["session_id"].(string), s1["block_id"].(string)
	waitText(t, c, b1, "$")

	// Splits, by label and direction, with a ratio.
	sp := act(t, c, "block.split", Args{Session: "work", Direction: "below", Ratio: 0.3, Create: &CreateOptions{Label: "logs"}})
	b2 := sp["block_id"].(string)
	m := model(t, c)
	w := m.Sessions[0].Windows[0]
	if w.Layout.Vertical != true || w.Layout.B.Block != b2 || w.Layout.Ratio < 0.69 || w.Layout.Ratio > 0.71 {
		t.Fatalf("split below: %+v", w.Layout)
	}
	if w.Focus != b2 || m.Blocks[b2].Label != "logs" {
		t.Fatalf("focus %s, label %q", w.Focus, m.Blocks[b2].Label)
	}
	sp = act(t, c, "block.split", Args{Block: "logs", Direction: "right", NoFocus: true})
	b3 := sp["block_id"].(string)
	if m = model(t, c); m.Sessions[0].Windows[0].Focus != b2 {
		t.Errorf("--focus=false moved the focus to %s", m.Sessions[0].Windows[0].Focus)
	}

	// Focus by direction: up from logs is the first block.
	act(t, c, "focus", Args{Session: "work", Target: "up"})
	if m = model(t, c); m.Sessions[0].Windows[0].Focus != b1 {
		t.Errorf("focus up went to %s, not %s", m.Sessions[0].Windows[0].Focus, b1)
	}
	act(t, c, "focus", Args{Session: "work", Target: "logs"})
	act(t, c, "focus", Args{Session: "work", Target: "right"})
	if m = model(t, c); m.Sessions[0].Windows[0].Focus != b3 {
		t.Errorf("focus right went to %s, not %s", m.Sessions[0].Windows[0].Focus, b3)
	}

	// Zoom on and off; swap; move.
	act(t, c, "block.zoom", Args{Block: b3, Zoom: "on"})
	if m = model(t, c); m.Sessions[0].Windows[0].Zoom != b3 {
		t.Error("not zoomed")
	}
	act(t, c, "block.zoom", Args{Block: b3, Zoom: "off"})
	act(t, c, "block.swap", Args{Block: b1, Target: b3})
	if got := model(t, c).Sessions[0].Windows[0].Layout.Blocks(); !slices.Equal(got, []string{b3, b2, b1}) {
		t.Errorf("after swap %v", got)
	}
	act(t, c, "block.move", Args{Block: b1, Direction: "left", Target: b3})
	if got := model(t, c).Sessions[0].Windows[0].Layout.Blocks(); !slices.Equal(got, []string{b1, b3, b2}) {
		t.Errorf("after move %v", got)
	}
	var err error
	if err = c.Do("block.move", &Args{Block: b1, Direction: "left", Target: b1}, nil); err == nil {
		t.Error("moved next to itself")
	} else if e := (*Error)(nil); !errors.As(err, &e) || e.Code != "usage" {
		t.Errorf("error %v", err)
	}

	// Windows: a new one after the active, renamed, moved, focused by
	// position.
	nw := act(t, c, "window.new", Args{Session: sid, WindowLabel: "build"})
	wid := nw["window_id"].(string)
	act(t, c, "window.new", Args{Session: sid, WindowLabel: "tests", NoFocus: true})
	m = model(t, c)
	if ss := m.Sessions[0]; len(ss.Windows) != 3 || ss.Active != wid || ss.Windows[2].Label != "tests" {
		t.Fatalf("windows %+v active %s", ss.Windows, ss.Active)
	}
	act(t, c, "window.move", Args{Session: sid, Window: "tests", Index: 1})
	act(t, c, "window.focus", Args{Session: sid, Target: "1"})
	if ss := model(t, c).Sessions[0]; ss.Windows[0].Label != "tests" || ss.Active != ss.Windows[0].ID {
		t.Errorf("window 1 is %q, active %s", ss.Windows[0].Label, ss.Active)
	}

	// A second session; targets in it, and the ambiguity of a label in
	// both.
	s2 := act(t, c, "session.new", Args{Label: "play"})
	act(t, c, "block.rename", Args{Block: s2["block_id"].(string), Label: "logs"})
	if err = c.Do("block.focus", &Args{Block: "logs"}, nil); err == nil {
		// The default session is the first, where "logs" is one block.
	} else {
		t.Errorf("logs in the default session: %v", err)
	}
	if err = c.Do("block.focus", &Args{Block: "logs", Session: "play"}, nil); err != nil {
		t.Errorf("logs in play: %v", err)
	}
	act(t, c, "session.move", Args{Session: "play", Before: "work"})
	if ss := model(t, c).Sessions; ss[0].Label != "play" {
		t.Errorf("sessions %s %s", ss[0].Label, ss[1].Label)
	}
	if err = c.Do("session.kill", &Args{Session: "nope"}, nil); err == nil {
		t.Error("killed no session")
	} else if e := (*Error)(nil); !errors.As(err, &e) || e.Code != "not_found" {
		t.Errorf("error %v", err)
	}

	// Closing the last block of a window closes it; of a session, ends it.
	act(t, c, "block.close", Args{Block: s2["block_id"].(string)})
	if ss := model(t, c).Sessions; len(ss) != 1 {
		t.Errorf("%d sessions after closing play's only block", len(ss))
	}
}

// spec: F-112, F-142, F-143, F-144
func TestFocusFromContextAndZoom(t *testing.T) {
	c, _ := startServer(t)
	s1 := act(t, c, "session.new", Args{Label: "work"})
	left := s1["block_id"].(string)
	right := act(t, c, "block.split", Args{Block: left, Direction: "right"})["block_id"].(string)
	far := act(t, c, "block.split", Args{Block: right, Direction: "right"})["block_id"].(string)
	focus := func() string { return model(t, c).Sessions[0].Windows[0].Focus }

	// The focus is on far; rex runs in left: right of left is right.
	act(t, c, "focus", Args{Target: "right", CtxBlock: left})
	if got := focus(); got != right {
		t.Errorf("right of the context block went to %s, not %s", got, right)
	}
	// Without a context block, a direction starts from the focused one.
	act(t, c, "focus", Args{Target: "right"})
	if got := focus(); got != far {
		t.Errorf("right of the focused block went to %s, not %s", got, far)
	}
	// With -w, from the window's focused block, not the context block.
	act(t, c, "focus", Args{Target: "left", Session: "work", CtxBlock: left})
	if got := focus(); got != right {
		t.Errorf("left of the window's focused block went to %s, not %s", got, right)
	}

	// Focusing another block, by any command, ends the zoom.
	for _, f := range []struct {
		name string
		a    Args
		want string
	}{
		{"focus left", Args{Target: "left"}, left},
		{"focus next", Args{Target: "next"}, far},
		{"focus BLOCK", Args{Target: left}, left},
	} {
		act(t, c, "block.focus", Args{Block: right})
		act(t, c, "block.zoom", Args{Block: right, Zoom: "on"})
		act(t, c, "focus", f.a)
		w := model(t, c).Sessions[0].Windows[0]
		if w.Focus != f.want || w.Zoom != "" {
			t.Errorf("%s in a zoomed window: focus %s, zoom %q; want %s, no zoom", f.name, w.Focus, w.Zoom, f.want)
		}
	}
	// Focusing the zoomed block keeps the zoom.
	act(t, c, "block.zoom", Args{Block: right, Zoom: "on"})
	act(t, c, "block.focus", Args{Block: right})
	if w := model(t, c).Sessions[0].Windows[0]; w.Zoom != right {
		t.Errorf("focusing the zoomed block ended its zoom: %q", w.Zoom)
	}
}

// spec: F-135, F-145, F-147, T-107
func TestRatioLimit(t *testing.T) {
	c, _ := startServer(t)
	b1 := act(t, c, "session.new", Args{})["block_id"].(string)
	ratio := func() float32 { return model(t, c).Sessions[0].Windows[0].Layout.Ratio }
	near := func(r, want float32) bool { return r > want-0.001 && r < want+0.001 }

	// A split of 3 % gives the new block 5 %.
	b2 := act(t, c, "block.split", Args{Block: b1, Direction: "right", Ratio: 0.03})["block_id"].(string)
	if r := ratio(); !near(r, MaxRatio) {
		t.Errorf("split of 3 %%: ratio %v, not %v", r, MaxRatio)
	}
	// Resizing stops at the limit, not at 10 or 90 %.
	for range 30 {
		act(t, c, "block.resize", Args{Block: b2, Direction: "left", Amount: 10})
	}
	if r := ratio(); !near(r, MinRatio) {
		t.Errorf("resized to the end: ratio %v, not %v", r, MinRatio)
	}
	// A move of 99 %, and a ratio the app sets, stay in the limit.
	act(t, c, "block.move", Args{Block: b2, Direction: "left", Target: b1, Ratio: 0.99})
	if r := ratio(); !near(r, MaxRatio) {
		t.Errorf("move of 99 %%: ratio %v, not %v", r, MaxRatio)
	}
	split := model(t, c).Sessions[0].Windows[0].Layout.Split
	act(t, c, "split.ratio", Args{Split: split, Ratio: 0.01})
	if r := ratio(); !near(r, MinRatio) {
		t.Errorf("split.ratio 0.01: ratio %v, not %v", r, MinRatio)
	}
	if ClampRatio(0.08) != 0.08 || ClampRatio(0.97) != MaxRatio {
		t.Error("ClampRatio, which the app's dragged dividers use, holds another limit")
	}
}

// spec: F-139, F-160, F-161, F-164, F-166
func TestSendCaptureWait(t *testing.T) {
	c, _ := startServer(t)
	s := act(t, c, "session.new", Args{Create: &CreateOptions{Dir: "/tmp"}})
	b := s["block_id"].(string)
	waitText(t, c, b, "$")
	act(t, c, "send", Args{Block: b, Data: "echo he''llo-$((40+2))"})
	act(t, c, "sendKey", Args{Block: b, Keys: []string{"enter"}})
	waitText(t, c, b, "hello-42")

	// Ctrl+C interrupts.
	act(t, c, "send", Args{Block: b, Data: "sleep 30\r"})
	time.Sleep(300 * time.Millisecond)
	act(t, c, "sendKey", Args{Block: b, Keys: []string{"ctrl+c"}})
	act(t, c, "send", Args{Block: b, Data: "echo after\r"})
	waitText(t, c, b, "after")

	// A command's exit status, waited for after it closed its block, and
	// a block kept open.
	sp := act(t, c, "block.split", Args{Block: b, Create: &CreateOptions{Command: []string{"sh", "-c", "exit 3"}}})
	var w map[string]any
	if err := c.DoFor("wait", &Args{Block: sp["block_id"].(string)}, &w, 5*time.Second); err != nil || w["exit_code"] != float64(3) {
		t.Errorf("wait %v %v", w, err)
	}
	sp = act(t, c, "block.split", Args{Block: b, Create: &CreateOptions{Command: []string{"sh", "-c", "echo kept; exit 5"}, KeepOpen: true}})
	kept := sp["block_id"].(string)
	if err := c.DoFor("wait", &Args{Block: kept}, &w, 5*time.Second); err != nil || w["exit_code"] != float64(5) {
		t.Errorf("wait kept %v %v", w, err)
	}
	if text := waitText(t, c, kept, "exited with code 5"); !strings.Contains(text, "kept") {
		t.Errorf("kept block shows %q", text)
	}
	if m := model(t, c); m.Blocks[kept] == nil || !m.Blocks[kept].Exited {
		t.Error("kept block gone or not exited")
	}
}

// spec: F-169, F-170, F-175, T-106, T-109
func TestEventsAndClientActions(t *testing.T) {
	c, _ := startServer(t)
	s := act(t, c, "session.new", Args{Label: "ev"})

	// A second connection follows the session.
	ev, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	got := make(chan Event, 64)
	ev.OnEvent = func(e Event) { got <- e }
	if err := ev.Subscribe(Args{Session: "ev"}); err != nil {
		t.Fatal(err)
	}
	act(t, c, "block.split", Args{Session: "ev"})
	act(t, c, "send", Args{Session: "ev", Data: "printf '\\a'\r"})
	want := map[string]bool{"block_created": false, "bell": false}
	deadline := time.After(5 * time.Second)
	for !want["block_created"] || !want["bell"] {
		select {
		case e := <-got:
			inner, _ := e.Payload["event"].(map[string]any)
			typ, _ := inner["type"].(string)
			if typ == "block_event" {
				p, _ := inner["payload"].(map[string]any)
				typ, _ = p["name"].(string)
			}
			want[typ] = true
		case <-deadline:
			t.Fatalf("events seen %v", want)
		}
	}

	// An app takes client actions.
	app, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	app.OnEvent = func(e Event) {
		if e.Type != "client_action" {
			return
		}
		id := int64(e.Payload["action_id"].(float64))
		go app.Reply(id, map[string]any{"did": e.Payload["name"]}, nil)
	}
	if _, err := app.HelloAs("app", "test app"); err != nil {
		t.Fatal(err)
	}
	app.Subscribe(Args{Model: true})
	var res json.RawMessage
	if err := c.DoFor("client.do", &Args{Action: "client.tab.next"}, &res, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res), "client.tab.next") {
		t.Errorf("result %s", res)
	}
	_ = s
}

// spec: F-182
func TestRestoreAfterRestart(t *testing.T) {
	c, dir := startServer(t)
	s := act(t, c, "session.new", Args{Label: "keep", Create: &CreateOptions{Dir: "/tmp"}})
	b := s["block_id"].(string)
	low := act(t, c, "block.split", Args{Block: b, Direction: "below", Create: &CreateOptions{Label: "lower", Dir: "/usr"}})
	act(t, c, "window.new", Args{Session: "keep", WindowLabel: "second"})
	waitText(t, c, low["block_id"].(string), "$")
	before := model(t, c)

	// The server ends, keeping its state; the next one starts the blocks
	// again in their directories, with their IDs and labels.
	c.Shutdown()
	for i := 0; i < 300; i++ {
		if _, err := os.Stat(filepath.Join(dir, "server.sock")); err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	// The next takes the lock once the last let it go.
	go func() {
		for Serve() != nil {
			time.Sleep(20 * time.Millisecond)
		}
	}()
	var c2 *Client
	var err error
	for i := 0; i < 300; i++ {
		if _, err = os.Stat(filepath.Join(dir, "server.sock")); err == nil {
			if c2, err = Connect(); err == nil {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer c2.ShutdownForget()
	after := model(t, c2)
	if len(after.Sessions) != 1 || after.Sessions[0].Label != "keep" || len(after.Sessions[0].Windows) != 2 {
		t.Fatalf("restored %+v", after.Sessions)
	}
	bw, aw := before.Sessions[0].Windows[0], after.Sessions[0].Windows[0]
	if !slices.Equal(bw.Layout.Blocks(), aw.Layout.Blocks()) || aw.Layout.Vertical != true {
		t.Errorf("layout %v, was %v", aw.Layout.Blocks(), bw.Layout.Blocks())
	}
	lb := after.Blocks[low["block_id"].(string)]
	if lb == nil || lb.Label != "lower" || !lb.Restored {
		t.Fatalf("lower block %+v", lb)
	}
	var infos []BlockInfo
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		infos, _ = c2.List()
		if i := slices.IndexFunc(infos, func(in BlockInfo) bool { return in.ID == lb.ID }); i >= 0 && infos[i].Dir == "/usr" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("the lower block is not back in /usr: %+v", infos)
}

func TestMigrateLayout(t *testing.T) {
	old := `{"active":1,"tabs":[{"root":{"ratio":0.3,"a":{"sid":"aa","dir":"/tmp"},"b":{"sid":"bb","dir":"/usr"}},"focus":1},{"name":"logs","root":{"sid":"cc","dir":"/"},"focus":0}]}`
	st := migrateLayout([]byte(old))
	if len(st.Sessions) != 1 {
		t.Fatalf("%+v", st)
	}
	ws := st.Sessions[0].Windows
	if len(ws) != 2 || ws[1].Label != "logs" || st.Sessions[0].Active != ws[1].ID {
		t.Fatalf("windows %+v", ws)
	}
	if bs := ws[0].Layout.Blocks(); len(bs) != 2 || ws[0].Focus != bs[1] || st.Blocks[bs[1]].Dir != "/usr" || ws[0].Layout.Ratio != 0.3 {
		t.Errorf("first window %+v", ws[0])
	}
}

// session_destroyed carries the session's ID, so that a subscription to
// one session hears its end.
//
// spec: F-169, T-106, T-161
func TestSessionDestroyedReachesItsSession(t *testing.T) {
	c, _ := startServer(t)
	s := act(t, c, "session.new", Args{Label: "end"})
	ev, err := Connect()
	if err != nil {
		t.Fatal(err)
	}
	defer ev.Close()
	got := make(chan Event, 64)
	ev.OnEvent = func(e Event) { got <- e }
	if err := ev.Subscribe(Args{Session: "end"}); err != nil {
		t.Fatal(err)
	}
	act(t, c, "session.kill", Args{Session: "end"})
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-got:
			if e.Type != "session_destroyed" {
				continue
			}
			if e.SessionID != s["session_id"] || e.Payload["session_id"] != s["session_id"] {
				t.Errorf("session_destroyed = %+v", e)
			}
			return
		case <-deadline:
			t.Fatal("no session_destroyed for the session followed")
		}
	}
}
