package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"

	"gorex/internal/rex"
)

// The app mirrors the server's model: the sessions, their windows (the
// app's tabs) and the layouts of their blocks (the app's panes). What the
// user does goes to the server, whose model comes back and the view shows;
// the CLI changes the same model, so the window follows it.

// Tab is a window of the session the app shows: panes in a tree of
// splits.
type Tab struct {
	ID   int    // the tab's key in the view
	WID  string // the window's ID
	Name string // the label the user gave it, "" to follow its panes
	Root *Node
	// Focus is the pane with the focus, and Zoom the pane that fills the
	// tab, when one does.
	Focus *Pane
	Zoom  *Pane
}

// Node is a pane, or a split of two nodes.
type Node struct {
	ID      int
	SplitID string
	Pane    *Pane
	Parent  *Node
	// Vertical stacks A above B; otherwise A is left of B. Ratio is A's
	// share of the room.
	Vertical bool
	Ratio    float32
	A, B     *Node
}

// Pane is a terminal attached to a block of the server.
type Pane struct {
	ID     int
	SID    string // the block's ID
	Tab    *Tab
	Node   *Node
	term   *terminal.Terminal
	stream *rex.Stream

	// meta is what the model says of the block, and info what the server
	// last said runs in it.
	meta rex.BlockMeta
	info rex.BlockInfo
	// title is the last title the program set.
	title string
	// lastData is when the block last printed, in Unix nanoseconds.
	lastData atomic.Int64
	// attention marks a pane that rang the bell or whose program finished
	// while it was out of sight; seen clears it.
	attention bool
	// bounds is where the pane's card was in the last frame.
	bounds ui.Rect
	closed bool
	// statusSeen is when the user last looked at the pane, which a result
	// it reported before is seen; lastNotify when it last notified.
	statusSeen, lastNotify time.Time
}

// App is the state of GoRex's window.
type App struct {
	win    *mygo.Window
	client *rex.Client
	hello  rex.Hello
	err    string

	// model is the server's, and session the ID of the session the
	// window shows, whose windows are tabs.
	model   *rex.Model
	session string
	tabs    []*Tab
	active  int
	panes   map[string]*Pane
	tabOf   map[string]*Tab
	keyOf   map[string]int
	nextID  int
	// dragging is the split whose divider the pointer drags: its ratio is
	// the view's until the drag ends.
	dragging string
	ratios   map[string]float32

	// focusReq asks the view to give the keyboard focus to a pane's
	// terminal.
	focusReq *Pane

	// The command palette, its mode, the host's popover and the tab being
	// renamed.
	paletteOpen  bool
	paletteMode  paletteMode
	paletteQuery string
	paletteSel   int
	prompt       *prompt
	hostOpen     bool
	renaming     *Tab
	renameText   string
	renameFocus  bool
	quitting     bool
	focusedWin   bool
	title        string

	// keySeq is the sequence of keys begun, and keySeqAt when.
	keySeq   []chord
	keySeqAt time.Time

	// posted are changes to make in the next frame, from terminals of a
	// view without a window, as in tests.
	postMu sync.Mutex
	posted []func()
	// services keeps redraw requests available to menu callbacks.
	services ui.Services
}

func (a *App) post(fn func()) {
	a.postMu.Lock()
	a.posted = append(a.posted, fn)
	a.postMu.Unlock()
}

// update changes the state on the main thread: the window's, or the next
// frame's without one.
func (a *App) update(fn func()) {
	if win := a.win; win != nil {
		win.Update(fn)
	} else {
		a.postMu.Lock()
		a.posted = append(a.posted, fn)
		sv := a.services
		a.postMu.Unlock()
		sv.Invalidate() // nothing without a view

	}
}

// later changes the tree of tabs and panes once the frame building now
// is built, as closing what it is building would pull it from under it.
func (a *App) later(c *ui.Context, fn func()) {
	a.post(fn)
	c.Invalidate()
}

// runPosted makes the changes posted since the last frame.
func (a *App) runPosted() {
	a.postMu.Lock()
	fns := a.posted
	a.posted = nil
	a.postMu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

func (a *App) id() int {
	a.nextID++
	return a.nextID
}

// key returns a key of the view that stays the same for an ID of the
// model.
func (a *App) key(id string) int {
	if a.keyOf == nil {
		a.keyOf = map[string]int{}
	}
	k, ok := a.keyOf[id]
	if !ok {
		k = a.id()
		a.keyOf[id] = k
	}
	return k
}

func (a *App) tab() *Tab {
	if a.active >= 0 && a.active < len(a.tabs) {
		return a.tabs[a.active]
	}
	return nil
}

// panes returns the panes of a node in order, left to right and top to
// bottom.
func (n *Node) panes() []*Pane {
	if n == nil {
		return nil
	}
	if n.Pane != nil {
		return []*Pane{n.Pane}
	}
	return append(n.A.panes(), n.B.panes()...)
}

func (t *Tab) panes() []*Pane { return t.Root.panes() }

// connect connects to the server as the app, and mirrors its model from
// then on.
func (a *App) connect(client *rex.Client) error {
	a.client = client
	client.OnEvent = func(ev rex.Event) {
		switch ev.Type {
		case "model":
			m := ev.Model
			a.update(func() {
				if a.client == client {
					a.applyModel(m)
				}
			})
		case "client_action":
			id, _ := ev.Payload["action_id"].(float64)
			name, _ := ev.Payload["name"].(string)
			params, _ := json.Marshal(ev.Payload["params"])
			a.update(func() {
				res, err := a.clientAction(name, params)
				go client.Reply(int64(id), res, err)
			})
		}
	}
	h, err := client.HelloAs("app", appName())
	if err != nil {
		return err
	}
	a.hello = h
	m, err := client.Model()
	if err != nil {
		return err
	}
	a.applyModel(m)
	return client.Subscribe(rex.Args{Model: true})
}

func appName() string {
	host, _ := os.Hostname()
	return "GoRex on " + strings.TrimSuffix(host, ".local")
}

// act performs an action of the model on the server and takes the model
// it returns; a failure shows in the window.
func (a *App) act(op string, args rex.Args) json.RawMessage {
	if a.client == nil {
		return nil
	}
	res, m, err := a.client.Act(op, &args)
	if err != nil {
		log.Printf("%s: %v", op, err)
		a.err = err.Error()
		return nil
	}
	a.applyModel(m)
	return res
}

// applyModel shows the model: the panes of every block, the tabs of the
// session shown.
func (a *App) applyModel(m *rex.Model) {
	if m == nil || a.model != nil && m.Revision < a.model.Revision {
		return
	}
	a.model = m
	if a.panes == nil {
		a.panes, a.tabOf = map[string]*Pane{}, map[string]*Tab{}
	}
	for id, meta := range m.Blocks {
		p := a.panes[id]
		if p == nil {
			p = &Pane{ID: a.key(id), SID: id, info: rex.BlockInfo{ID: id, Shell: meta.Shell, Program: meta.Shell, Idle: true}}
			a.panes[id] = p
			a.attach(p, meta.Cols, meta.Rows)
		}
		p.meta = *meta
		p.info.Label = meta.Label
	}
	for id, p := range a.panes {
		if m.Blocks[id] == nil {
			p.closed = true
			if p.term != nil {
				p.term.Close() // detaches; the server ended the block
			}
			delete(a.panes, id)
			delete(a.keyOf, id)
		}
	}
	if m.Session(a.session) == nil {
		// The session shown ended: show the one before it, or the first.
		a.session = ""
		if len(m.Sessions) > 0 {
			a.session = m.Sessions[0].ID
		}
		if a.client != nil && a.session != "" {
			go a.client.View(a.session)
		}
	}
	a.rebuild()
	if len(m.Sessions) == 0 && a.win != nil && !a.quitting {
		a.win.Close()
	}
}

// rebuild makes the tabs of the session shown from the model.
func (a *App) rebuild() {
	was, wasFocus := a.tab(), (*Pane)(nil)
	if was != nil {
		wasFocus = was.Focus
	}
	ss := (*rex.Session)(nil)
	if a.model != nil {
		ss = a.model.Session(a.session)
	}
	for _, p := range a.panes {
		p.Tab, p.Node = nil, nil
	}
	var tabs []*Tab
	keep := map[string]bool{}
	active := 0
	if ss != nil {
		for i, w := range ss.Windows {
			t := a.tabOf[w.ID]
			if t == nil {
				t = &Tab{ID: a.key(w.ID), WID: w.ID}
				a.tabOf[w.ID] = t
			}
			keep[w.ID] = true
			t.Name = w.Label
			t.Root = a.buildNode(t, w.Layout, nil)
			ps := t.panes()
			t.Focus = a.panes[w.Focus]
			if t.Focus == nil || t.Focus.Tab != t {
				t.Focus = nil
				if len(ps) > 0 {
					t.Focus = ps[0]
				}
			}
			t.Zoom = nil
			if z := a.panes[w.Zoom]; z != nil && z.Tab == t && len(ps) > 1 {
				t.Zoom = z
			}
			tabs = append(tabs, t)
			if w.ID == ss.Active {
				active = i
			}
		}
	}
	for wid := range a.tabOf {
		if !keep[wid] {
			if a.renaming == a.tabOf[wid] {
				a.renaming = nil
			}
			delete(a.tabOf, wid)
		}
	}
	a.tabs, a.active = tabs, active
	// The keyboard follows the focus when it moved: by the CLI, or as the
	// user acted.
	if t := a.tab(); t != nil && (t != was || t.Focus != wasFocus) && !a.paletteOpen && a.renaming == nil {
		a.focusReq = t.Focus
		if t.Focus != nil {
			t.Focus.attention = false
		}
	}
}

// buildNode makes the tree of a tab's panes from a layout.
func (a *App) buildNode(t *Tab, l *rex.Layout, parent *Node) *Node {
	if l == nil {
		return nil
	}
	if l.IsLeaf() {
		n := &Node{ID: a.key("n:" + l.Block), Parent: parent}
		if p := a.panes[l.Block]; p != nil {
			n.Pane, p.Tab, p.Node = p, t, n
		} else {
			// A block of the layout the model has no word of yet.
			p := &Pane{ID: a.key(l.Block), SID: l.Block, Tab: t, Node: n, closed: true}
			n.Pane = p
		}
		return n
	}
	n := &Node{ID: a.key(l.Split), SplitID: l.Split, Parent: parent, Vertical: l.Vertical, Ratio: l.Ratio}
	if r, ok := a.ratios[l.Split]; ok && a.dragging == l.Split {
		n.Ratio = r
	}
	n.A, n.B = a.buildNode(t, l.A, n), a.buildNode(t, l.B, n)
	return n
}

// attach makes the pane's terminal, attached to its block.
func (a *App) attach(p *Pane, cols, rows int) {
	if a.client == nil {
		return
	}
	if cols <= 0 || rows <= 0 {
		cols, rows = 80, 24
	}
	p.stream = a.client.Stream(p.SID, cols, rows)
	p.stream.OnData = func(int) { p.lastData.Store(time.Now().UnixNano()) }
	term, err := terminal.New(terminal.Options{
		Conn:        p.stream,
		Font:        termFont,
		Theme:       lightTerm,
		DarkTheme:   darkTerm,
		Transparent: true,
		OnTitle: func(title string) {
			a.update(func() { p.title = title })
		},
		OnBell: func() {
			a.update(func() { a.ring(p) })
		},
		OnNotify: func(title, body string) {
			if title == "" {
				title = "GoRex"
			}
			mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body}).Show()
		},
		// The server closes a block whose program ended, and the model
		// says so.
	})
	if err != nil {
		log.Printf("terminal: %v", err)
		a.err = err.Error()
		return
	}
	// Until a view shows it, the terminal has the block's size, so that
	// attaching changes nothing in the block.
	term.Resize(cols, rows)
	p.term = term
}

func (a *App) ring(p *Pane) {
	if t := a.tab(); t == nil || t.Focus != p || p.Tab != t || !a.focusedWin {
		p.attention = true
	}
}

// Actions of the user, which the server performs.

// newTab opens a tab with a shell in dir, after the active tab.
func (a *App) newTab(dir string) *Tab {
	if a.session == "" {
		a.newSession()
		return a.tab()
	}
	res := a.act("window.new", rex.Args{Session: a.session, Create: &rex.CreateOptions{Dir: dir}})
	var r struct {
		Window string `json:"window_id"`
	}
	_ = json.Unmarshal(res, &r) // no window: nil
	return a.tabOf[r.Window]
}

// currentDir returns the directory new panes open in: the focused pane's.
func (a *App) currentDir() string {
	if t := a.tab(); t != nil && t.Focus != nil {
		if d := t.Focus.info.Dir; d != "" {
			return d
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

// split splits the focused pane, the new pane right of it or below it.
func (a *App) split(vertical bool) {
	if vertical {
		a.splitDir("below")
	} else {
		a.splitDir("right")
	}
}

// splitDir splits the focused pane toward a direction.
func (a *App) splitDir(dir string) *Pane {
	t := a.tab()
	if t == nil || t.Focus == nil {
		return nil
	}
	res := a.act("block.split", rex.Args{Block: t.Focus.SID, Direction: dir})
	var r struct {
		Block string `json:"block_id"`
	}
	_ = json.Unmarshal(res, &r) // no block: nil
	return a.panes[r.Block]
}

// setFocus gives a pane the focus of its tab, and of the server's model.
func (a *App) setFocus(p *Pane) {
	if p == nil || p.Tab == nil {
		return
	}
	p.attention = false
	if p.Tab.Focus == p && a.tab() == p.Tab {
		return
	}
	p.Tab.Focus = p
	a.act("block.focus", rex.Args{Block: p.SID})
}

// closePane closes a pane and ends its block.
func (a *App) closePane(p *Pane) {
	if p.closed {
		return
	}
	a.act("block.close", rex.Args{Block: p.SID})
}

func (a *App) closeFocused() {
	if t := a.tab(); t != nil && t.Focus != nil {
		a.closePane(t.Focus)
	}
}

func (a *App) closeOtherPanes() {
	t := a.tab()
	if t == nil || t.Focus == nil {
		return
	}
	for _, p := range t.panes() {
		if p != t.Focus {
			a.act("block.close", rex.Args{Block: p.SID})
		}
	}
}

// closeTab closes a tab and ends the blocks of its panes.
func (a *App) closeTab(t *Tab) { a.act("window.close", rex.Args{Window: t.WID}) }

func (a *App) closeActiveTab() {
	if t := a.tab(); t != nil {
		a.closeTab(t)
	}
}

func (a *App) closeOtherTabs() {
	t := a.tab()
	for _, o := range slices.Clone(a.tabs) {
		if o != t {
			a.closeTab(o)
		}
	}
}

func (a *App) selectTab(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	t := a.tabs[i]
	if t.Focus != nil {
		t.Focus.attention = false
	}
	a.act("window.focus", rex.Args{Window: t.WID})
	a.focusReq = t.Focus
}

// showTab shows the nth tab, from 1; 9 is the last.
func (a *App) showTab(n int) {
	if n == 9 {
		a.selectTab(len(a.tabs) - 1)
	} else {
		a.selectTab(n - 1)
	}
}

func (a *App) cycleTab(d int) {
	if n := len(a.tabs); n > 1 {
		a.selectTab(((a.active+d)%n + n) % n)
	}
}

func (a *App) moveTab(t *Tab, to int) {
	if from := slices.Index(a.tabs, t); from < 0 || to == from {
		return
	}
	a.act("window.move", rex.Args{Window: t.WID, Index: to + 1})
}

func (a *App) moveActiveTab(d int) {
	if t := a.tab(); t != nil && a.active+d >= 0 && a.active+d < len(a.tabs) {
		a.moveTab(t, a.active+d)
	}
}

func (a *App) renameTab(t *Tab, name string) {
	a.act("window.rename", rex.Args{Window: t.WID, Label: name})
}

// moveFocus gives the focus to the pane next to the focused one, toward
// dx, dy.
func (a *App) moveFocus(dx, dy int) {
	switch {
	case dx < 0:
		a.focusDir("left")
	case dx > 0:
		a.focusDir("right")
	case dy < 0:
		a.focusDir("up")
	default:
		a.focusDir("down")
	}
}

// focusDir moves the focus toward a direction: left, right, up, down,
// next or prev.
func (a *App) focusDir(dir string) {
	t := a.tab()
	if t == nil || t.Focus == nil {
		return
	}
	a.act("block.focus", rex.Args{Window: t.WID, Direction: dir})
}

// swapDir swaps the focused pane with the one next to it.
func (a *App) swapDir(dir string) {
	if t := a.tab(); t != nil && t.Focus != nil {
		a.act("block.swap", rex.Args{Block: t.Focus.SID, Direction: dir})
	}
}

// resizeFocused moves the nearest divider of the focused pane on the
// axis of dx, dy.
func (a *App) resizeFocused(dx, dy int) {
	switch {
	case dx < 0:
		a.resizeDir("left", 0)
	case dx > 0:
		a.resizeDir("right", 0)
	case dy < 0:
		a.resizeDir("up", 0)
	default:
		a.resizeDir("down", 0)
	}
}

func (a *App) resizeDir(dir string, amount float32) {
	if t := a.tab(); t != nil && t.Focus != nil {
		res, m, err := a.client.Act("block.resize", &rex.Args{Block: t.Focus.SID, Direction: dir, Amount: amount})
		_ = res
		if err == nil {
			a.applyModel(m)
		}
	}
}

// setRatio sets a split's ratio as its divider is dragged: the view's at
// once, the server's soon.
func (a *App) setRatio(n *Node, ratio float32, done bool) {
	n.Ratio = ratio
	if a.ratios == nil {
		a.ratios = map[string]float32{}
	}
	a.ratios[n.SplitID] = ratio
	a.dragging = n.SplitID
	if done {
		a.dragging = ""
	}
	c := a.client
	id := n.SplitID
	go func() {
		_, m, err := c.Act("split.ratio", &rex.Args{Split: id, Ratio: ratio})
		if err == nil {
			a.update(func() { a.applyModel(m) })
		}
	}()
}

// equalize gives every split of the tab equal shares.
func (a *App) equalize() {
	if t := a.tab(); t != nil {
		a.act("window.balance", rex.Args{Window: t.WID})
	}
}

func (a *App) toggleZoom() {
	t := a.tab()
	if t == nil || t.Focus == nil {
		return
	}
	if t.Zoom == nil && len(t.panes()) < 2 {
		return
	}
	a.act("block.zoom", rex.Args{Block: t.Focus.SID})
	a.focusReq = t.Focus
}

func (a *App) breakPane() {
	if t := a.tab(); t != nil && t.Focus != nil {
		a.act("block.break", rex.Args{Block: t.Focus.SID})
	}
}

func (a *App) copyPaneID() {
	if t := a.tab(); t != nil && t.Focus != nil {
		mygo.Clipboard.WriteText(t.Focus.SID)
	}
}

func (a *App) renamePane() {
	t := a.tab()
	if t == nil || t.Focus == nil {
		return
	}
	p := t.Focus
	a.openPrompt("Rename Pane", "A label for the pane, which rex commands can target", p.meta.Label, func(s string) {
		a.act("block.rename", rex.Args{Block: p.SID, Label: s})
	})
}

func (a *App) clearFocused() {
	if t := a.tab(); t != nil && t.Focus != nil && t.Focus.term != nil {
		// Clear the scrollback and the screen, and have the shell draw
		// its prompt again, as Command-K does in Terminal.
		t.Focus.term.Feed([]byte("\x1b[H\x1b[2J\x1b[3J"))
		if t.Focus.info.Idle {
			t.Focus.term.Send([]byte{0x0c})
		}
	}
}

// restartFocused ends the focused pane's block and starts a shell in its
// place, in the same directory.
func (a *App) restartFocused() {
	if t := a.tab(); t != nil && t.Focus != nil {
		a.act("block.restart", rex.Args{Block: t.Focus.SID})
	}
}

func (a *App) quitAndEnd() {
	a.quitting = true
	for _, p := range a.panes {
		p.closed = true
	}
	c := a.client
	go func() {
		c.ShutdownForget()
		mygo.App.Quit()
	}()
}

// Sessions.

// newSession starts a session, with a shell in the focused pane's
// directory, and shows it.
func (a *App) newSession() {
	res := a.act("session.new", rex.Args{Create: &rex.CreateOptions{Dir: a.currentDir()}})
	var r struct {
		Session string `json:"session_id"`
	}
	if json.Unmarshal(res, &r) == nil && r.Session != "" {
		a.showSession(r.Session)
	}
}

// showSession shows a session's tabs in the window.
func (a *App) showSession(id string) {
	if a.model == nil || a.model.Session(id) == nil {
		return
	}
	a.session = id
	a.rebuild()
	if t := a.tab(); t != nil {
		a.focusReq = t.Focus
	}
	if c := a.client; c != nil {
		go c.View(id)
	}
}

// selectSession shows a session by target: an ID, label, position or the
// end of an ID.
func (a *App) selectSession(target string) error {
	if a.model == nil {
		return fmt.Errorf("no sessions")
	}
	for i, ss := range a.model.Sessions {
		if ss.ID == target || strings.EqualFold(ss.Label, target) || fmt.Sprint(i+1) == target ||
			len(target) >= 4 && strings.HasSuffix(ss.ID, target) {
			a.showSession(ss.ID)
			return nil
		}
	}
	return fmt.Errorf("session %q not found", target)
}

func (a *App) cycleSession(d int) {
	if a.model == nil || len(a.model.Sessions) < 2 {
		return
	}
	i := slices.IndexFunc(a.model.Sessions, func(s *rex.Session) bool { return s.ID == a.session })
	n := len(a.model.Sessions)
	a.showSession(a.model.Sessions[((i+d)%n+n)%n].ID)
}

func (a *App) renameSession() {
	ss := a.currentSession()
	if ss == nil {
		return
	}
	id := ss.ID
	a.openPrompt("Rename Session", "A name for the session", ss.Label, func(s string) {
		a.act("session.rename", rex.Args{Session: id, Label: s})
	})
}

func (a *App) closeSession() {
	if ss := a.currentSession(); ss != nil {
		a.act("session.kill", rex.Args{Session: ss.ID})
	}
}

func (a *App) currentSession() *rex.Session {
	if a.model == nil {
		return nil
	}
	return a.model.Session(a.session)
}

// sessionName names the session shown.
func (a *App) sessionName(ss *rex.Session) string {
	if ss == nil || a.model == nil {
		return ""
	}
	return a.model.SessionName(ss)
}

// Labels.

// label returns how a pane names itself: its label, its program, or the
// title the program set, and the directory it works in.
func (p *Pane) label() (name, detail string) {
	in := p.info
	if p.meta.Exited {
		name, _ := p.programLabel()
		return name, fmt.Sprintf("exited (%d)", p.meta.ExitCode)
	}
	name, detail = p.programLabel()
	if l := p.meta.Label; l != "" {
		if name != "" && !strings.EqualFold(name, l) && !in.Idle {
			return l, name
		}
		return l, detail
	}
	return name, detail
}

func (p *Pane) programLabel() (name, detail string) {
	in := p.info
	prog := programOf(in.Program)
	dir := shortDir(in.Dir)
	if in.Program == "" {
		return in.Shell, ""
	}
	if in.Idle || prog.Shell {
		name := in.Program
		if name == "" {
			name = in.Shell
		}
		return name, dir
	}
	if t := strings.TrimSpace(p.title); t != "" && !titleOfShell(t, in) {
		return t, ""
	}
	name = prog.Name
	if name == "ssh" || in.Program == "ssh" {
		if host := sshHost(in.Args); host != "" {
			return "SSH", host
		}
	}
	return name, dir
}

// titleOfShell tells a title a shell set, naming the program or the
// directory, from one a program set of its own.
func titleOfShell(title string, in rex.BlockInfo) bool {
	l := strings.ToLower(title)
	for _, w := range []string{strings.ToLower(in.Program), strings.ToLower(in.Shell)} {
		if w != "" && strings.HasPrefix(l, w) {
			return true
		}
	}
	if home, _ := os.UserHomeDir(); strings.Contains(title, "~") || (home != "" && strings.Contains(title, home)) {
		return true
	}
	return strings.Contains(title, "@") && strings.Contains(title, ":")
}

func sshHost(args []string) string {
	for i := 1; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			if len(a) == 2 && strings.Contains("bcDEeFIiJLlmOopQRSWw", a[1:]) {
				i++
			}
			continue
		}
		return a
	}
	return ""
}

// shortDir shows a directory as shells do, from ~, and only its last
// parts when it is deep.
func shortDir(d string) string {
	if d == "" {
		return ""
	}
	sep := string(filepath.Separator)
	if home, _ := os.UserHomeDir(); home != "" {
		if d == home {
			return "~"
		}
		if len(d) > len(home) && d[:len(home)] == home && d[len(home)] == filepath.Separator {
			d = "~" + d[len(home):]
		}
	}
	parts := strings.Split(d, sep)
	if len(parts) > 5 || len(d) > 48 && len(parts) > 3 {
		return "…" + sep + strings.Join(parts[len(parts)-2:], sep)
	}
	return d
}

// label names a tab: its name, or its focused pane's; and what a pane
// of it waits for, when one does.
func (t *Tab) label() (name, detail string) {
	if b := t.blockedPane(); b != nil {
		if t.Name != "" {
			name = t.Name
		} else {
			name, _ = b.label()
		}
		return name, strings.ToLower(statusWords(b.programStatus()))
	}
	if t.Name != "" {
		return t.Name, ""
	}
	if t.Focus == nil {
		return "Shell", ""
	}
	return t.Focus.label()
}

// poll asks the server what runs in each block, every half second, until
// the window closes.
func (a *App) poll(win *mygo.Window, client *rex.Client) {
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-client.Closed():
			win.Update(func() {
				if !a.quitting {
					a.err = "Lost the connection to the session server."
				}
			})
			return
		case <-tick.C:
		}
		infos, err := client.List()
		if err != nil {
			continue
		}
		byID := make(map[string]rex.BlockInfo, len(infos))
		for _, in := range infos {
			byID[in.ID] = in
		}
		win.Update(func() {
			if a.win == win {
				a.apply(byID)
			}
		})
	}
}

// apply takes what the server said runs in the blocks.
func (a *App) apply(byID map[string]rex.BlockInfo) {
	shown := a.tab()
	for _, p := range a.panes {
		in, ok := byID[p.SID]
		if !ok {
			continue
		}
		was := p.info
		p.info = in
		a.statusChanged(p, was.Status, in.Status)
		// A program that ran a while and finished out of sight asks for
		// attention.
		if !was.Idle && was.Program != "" && in.Idle && was.Program != in.Program {
			seen := p.Tab != nil && p.Tab == shown && shown.Focus == p && a.focusedWin
			if !seen {
				p.attention = true
			}
			if !a.focusedWin && time.Since(was.LastInput) > 8*time.Second {
				prog := programOf(was.Program)
				n := mygo.NewNotification(mygo.NotificationOptions{
					Title: prog.Name + " finished",
					Body:  "in " + shortDir(in.Dir),
				})
				n.Show()
			}
		}
	}
}

// sessionAttention tells whether a session has a pane asking for
// attention, and whether one prints.
func (a *App) sessionActivity(c *ui.Context, ss *rex.Session) (attention, busy bool) {
	for _, w := range ss.Windows {
		for _, id := range w.Layout.Blocks() {
			p := a.panes[id]
			if p == nil {
				continue
			}
			switch a.statusOf(c, p) {
			case statusAttention:
				attention = true
			case statusRunning:
				busy = true
			}
		}
	}
	return
}

// laterService queues a menu action without keeping a build context.
func (a *App) laterService(fn func()) {
	a.postMu.Lock()
	a.posted = append(a.posted, fn)
	sv := a.services
	a.postMu.Unlock()
	sv.Invalidate()
}

func (a *App) startRename() {
	if t := a.tab(); t != nil {
		a.renaming, a.renameText, a.renameFocus = t, t.Name, false
		a.focusReq = nil // the field takes the focus
	}
}
