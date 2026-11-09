package rex

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/egoist/mygo/plugins/terminal"
	"golang.org/x/text/width"
)

// scrollback is about how many bytes of output a block keeps above its
// screen, for the windows that attach to it and for capture: about 64
// MiB, as Rex's terminals keep.
const scrollback = 64 << 20

// block is a pseudo-terminal of the server and its program.
type block struct {
	id       string
	shell    string
	command  []string
	created  time.Time
	keepOpen bool
	restored bool
	p        *pty

	// label is the name the user gave the block; the server's lock
	// guards it.
	label string

	mu sync.Mutex
	// vt is the block's screen, kept by the terminal emulator of the
	// app's terminals without a view: a window attaching gets a snapshot
	// of it at its size, as the block shows it now.
	vt         *terminal.Terminal
	vtIn       *io.PipeWriter
	output     uint64
	bells      atomic.Int64
	clients    map[*attached]struct{}
	lastOutput time.Time
	lastInput  time.Time
	resync     *time.Timer
	exited     bool
	code       int
	cols, rows int
	// appCursor tells that the program asked for application cursor keys
	// (DECCKM), for send-key's arrows.
	appCursor bool
	done      chan struct{}

	// status is what programs reported of themselves (OSC 7501), and sc
	// finds the sequences of the output that tell it.
	status statusSet
	sc     oscScanner

	// events are what the block tells the server: a bell, a title, a
	// notification, its program's exit.
	onExit   func(code int)
	onBell   func()
	onTitle  func(title string)
	onNotify func(title, body string)
	// onStatus hears a record changed, or records removed, and why.
	onStatus func(changed *StatusRecord, removed []string, reason string)
}

// attached is a connection attached to a block, whose output a goroutine
// writes so that a slow client never holds up the others.
type attached struct {
	conn net.Conn
	out  chan []byte
	once sync.Once
}

func (a *attached) send(p []byte) bool {
	select {
	case a.out <- p:
		return true
	default:
		return false // too far behind: drop it, it attaches again
	}
}

func (a *attached) finish() { a.once.Do(func() { close(a.out) }) }

func (a *attached) writer() {
	for p := range a.out {
		if _, err := a.conn.Write(p); err != nil {
			break
		}
	}
	a.conn.Close()
	for range a.out {
	}
}

// blockEnv is where a block is, for its environment: what tells the rex
// command, and agents, which session and block they run in.
type blockEnv struct {
	block, session string
}

func newBlock(id string, o CreateOptions, where blockEnv) (*block, error) {
	path, argv, name, err := shellCommand(o)
	if err != nil {
		return nil, err
	}
	cols, rows := o.Cols, o.Rows
	if cols <= 0 || rows <= 0 {
		cols, rows = 80, 24
	}
	dir := o.Dir
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	env := blockEnvironment(where, o.Env)
	p, err := startPTY(path, argv, dir, env, cols, rows)
	if err != nil {
		return nil, err
	}
	b := &block{
		id: id, shell: name, command: slices.Clone(o.Command), created: time.Now(), p: p,
		keepOpen: o.KeepOpen, label: o.Label,
		clients: map[*attached]struct{}{}, status: statusSet{},
		cols: cols, rows: rows, done: make(chan struct{}),
	}
	// The emulator's Conn reads nothing, and what it would answer programs
	// is dropped: the windows' terminals answer them.
	pr, pw := io.Pipe()
	vt, err := terminal.New(terminal.Options{
		Conn: discard{pr}, Scrollback: scrollback,
		OnBell: func() {
			b.bells.Add(1)
			if f := b.onBell; f != nil {
				f()
			}
		},
		OnTitle: func(t string) {
			if f := b.onTitle; f != nil {
				f(t)
			}
		},
		OnNotify: func(title, body string) {
			if f := b.onNotify; f != nil {
				f(title, body)
			}
		},
	})
	if err != nil {
		p.hangup()
		return nil, err
	}
	vt.Resize(cols, rows)
	b.vt, b.vtIn = vt, pw
	return b, nil
}

// start reads what the block's program prints and waits for it to end,
// once the server set the block's callbacks.
func (b *block) start() {
	read := make(chan struct{})
	go func() {
		b.read()
		close(read)
	}()
	go func() {
		code := b.p.wait()
		// What it printed last is still to read; then hang up whatever
		// else holds the terminal, as terminal apps do.
		select {
		case <-read:
		case <-time.After(150 * time.Millisecond):
			b.p.close()
			<-read
		}
		b.mu.Lock()
		b.exited, b.code = true, code
		gone := b.status.drop("working", "blocked")
		if b.keepOpen {
			// The block stays, showing how its program ended.
			msg := []byte(fmt.Sprintf("\r\n\x1b[2m[process exited with code %d]\x1b[0m\r\n", code))
			b.vt.Feed(msg)
			for a := range b.clients {
				a.send(msg)
			}
		}
		for a := range b.clients {
			a.finish()
		}
		clear(b.clients)
		b.mu.Unlock()
		close(b.done)
		if f := b.onStatus; f != nil && len(gone) > 0 {
			f(nil, gone, "exit")
		}
		if f := b.onExit; f != nil {
			f(code)
		}
		if !b.keepOpen {
			b.vtIn.Close()
			b.vt.Close()
		}
	}()
}

// blockEnvironment returns the environment of a block's program: the
// server's, without what other terminals set, and what terminal apps set,
// with Rex's variables, which the rex command takes its context from.
func blockEnvironment(where blockEnv, extra []string) []string {
	var env []string
	hasLang := false
	path := ""
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "TERM", k == "COLORTERM", k == "TERM_PROGRAM", k == "TERM_PROGRAM_VERSION",
			k == "TERM_SESSION_ID", k == "TMUX", k == "TMUX_PANE", k == "STY", k == "WINDOWID",
			k == "COLUMNS", k == "LINES", k == "SHLVL", k == "OLDPWD", k == "PWD", k == "_",
			strings.HasPrefix(k, "ITERM_"), strings.HasPrefix(k, "GHOSTTY_"), strings.HasPrefix(k, "KITTY_"),
			strings.HasPrefix(k, "VSCODE_"), strings.HasPrefix(k, "WEZTERM_"), strings.HasPrefix(k, "ALACRITTY_"),
			strings.HasPrefix(k, "GOREX_"), strings.HasPrefix(k, "MYGO_"), strings.HasPrefix(k, "REX_"):
			continue
		case k == "PATH":
			path = v
			continue
		case k == "LANG" || k == "LC_ALL" || k == "LC_CTYPE":
			hasLang = true
		}
		env = append(env, kv)
	}
	// The rex command of this server comes first, as Rex has it.
	bin := BinDir()
	if path == "" {
		path = bin
	} else {
		path = bin + string(os.PathListSeparator) + path
	}
	env = append(env,
		"PATH="+path,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"TERM_PROGRAM=GoRex",
		"TERM_PROGRAM_VERSION="+Version,
		"REX_SESSION="+where.session,
		"REX_BLOCK="+where.block,
		"REX_SERVER=unix:"+ListenPath(),
		"REX_BIN_DIR="+bin,
		// GOREX_DIR tells the rex command which server to talk to: a
		// development build's has a directory of its own.
		"GOREX_DIR="+Dir(),
	)
	if !hasLang {
		env = append(env, "LANG=en_US.UTF-8")
	}
	return append(env, extra...)
}

// The sequences of the application cursor keys, on and off.
var (
	decckmOn  = []byte("\x1b[?1h")
	decckmOff = []byte("\x1b[?1l")
)

func (b *block) read() {
	buf := make([]byte, 64<<10)
	for {
		n, err := b.p.read(buf)
		if n > 0 {
			data := append([]byte(nil), buf[:n]...)
			b.mu.Lock()
			b.vt.Feed(data)
			b.output += uint64(n)
			b.lastOutput = time.Now()
			if on, off := bytes.LastIndex(data, decckmOn), bytes.LastIndex(data, decckmOff); on >= 0 || off >= 0 {
				b.appCursor = on > off
			}
			for a := range b.clients {
				if !a.send(data) {
					a.finish()
					delete(b.clients, a)
				}
			}
			notes, query := b.takeSequences(b.sc.scan(data))
			b.mu.Unlock()
			if query {
				b.p.write([]byte(statusQueryReply))
			}
			b.tell(notes)
		}
		if err != nil {
			return
		}
	}
}

// statusNote is a change of the status records, for the server.
type statusNote struct {
	changed *StatusRecord
	removed []string
	reason  string
}

// takeSequences acts on the sequences of the output: reports, prompts,
// commands and resets. It returns what changed, and whether a program
// asked whether the protocol is supported. The lock is held.
func (b *block) takeSequences(evs []oscEvent) (notes []statusNote, query bool) {
	for _, ev := range evs {
		switch ev.kind {
		case "status":
			rec, q, err := parseStatus(ev.body)
			if q {
				query = true
				continue
			}
			if err != nil {
				continue
			}
			changed, removed := b.status.apply(rec, time.Now())
			notes = append(notes, statusNote{changed, removed, "report"})
		case "prompt":
			// A shell prompt: what was running is over.
			if gone := b.status.drop("working", "blocked"); len(gone) > 0 {
				notes = append(notes, statusNote{nil, gone, "prompt"})
			}
		case "command":
			// The user runs the next command: the results before it are
			// seen.
			if gone := b.status.drop("done", "error", "idle"); len(gone) > 0 {
				notes = append(notes, statusNote{nil, gone, "command"})
			}
		case "reset":
			if gone := b.status.drop(statusStates...); len(gone) > 0 {
				notes = append(notes, statusNote{nil, gone, "reset"})
			}
		}
	}
	return notes, query
}

// tell gives the server the changes of the status records.
func (b *block) tell(notes []statusNote) {
	if f := b.onStatus; f != nil {
		for _, n := range notes {
			if n.changed != nil || len(n.removed) > 0 {
				f(n.changed, n.removed, n.reason)
			}
		}
	}
}

// report takes a status report from a client, as from a program's hook
// that has no terminal to print it to.
func (b *block) report(body string) error {
	rec, q, err := parseStatus(body)
	if q {
		return nil
	}
	if err != nil {
		return err
	}
	b.mu.Lock()
	changed, removed := b.status.apply(rec, time.Now())
	b.mu.Unlock()
	b.tell([]statusNote{{changed, removed, "report"}})
	return nil
}

// dropStatus removes records of the given states, for a reason.
func (b *block) dropStatus(reason string, states ...string) {
	b.mu.Lock()
	gone := b.status.drop(states...)
	b.mu.Unlock()
	if len(gone) > 0 {
		b.tell([]statusNote{{nil, gone, reason}})
	}
}

// attach attaches a connection: it gets a snapshot of the block's screen
// and scrollback at its size, then what the block prints, and what it
// sends is typed.
func (b *block) attach(conn net.Conn, cols, rows int) {
	a := &attached{conn: conn, out: make(chan []byte, 2048)}
	b.mu.Lock()
	if cols > 0 && rows > 0 && !b.exited {
		// The screen reflows to the window's size, which the program
		// learns next.
		b.vt.Resize(cols, rows)
	}
	snap := b.vt.Snapshot()
	if title := b.vt.Title(); title != "" {
		snap = append([]byte("\x1b]2;"+title+"\x07"), snap...)
	}
	a.out <- snap
	exited := b.exited
	if exited {
		a.finish()
	} else {
		b.clients[a] = struct{}{}
	}
	b.mu.Unlock()
	go a.writer()
	if exited {
		return
	}
	if cols > 0 && rows > 0 {
		b.resize(cols, rows)
	}
	buf := make([]byte, 32<<10)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			b.input(buf[:n])
		}
		if err != nil {
			break
		}
	}
	b.mu.Lock()
	if _, ok := b.clients[a]; ok {
		delete(b.clients, a)
		a.finish()
	}
	b.mu.Unlock()
}

func (b *block) input(p []byte) error {
	b.mu.Lock()
	b.lastInput = time.Now()
	exited := b.exited
	b.mu.Unlock()
	if exited {
		return fmt.Errorf("the block's program has exited")
	}
	_, err := b.p.write(p)
	return err
}

// resize sets the size of the terminal and reports whether it changed.
func (b *block) resize(cols, rows int) bool {
	b.mu.Lock()
	changed := cols != b.cols || rows != b.rows
	b.cols, b.rows = cols, rows
	exited := b.exited
	if changed && !exited {
		b.vt.Resize(cols, rows)
		// Once the program drew its screen at the new size, the windows
		// get it again.
		if b.resync == nil {
			b.resync = time.AfterFunc(resyncDelay, b.resyncScreen)
		} else {
			b.resync.Reset(resyncDelay)
		}
	}
	b.mu.Unlock()
	if changed && !exited {
		b.p.resize(cols, rows)
	}
	return changed
}

// resyncDelay is how long after the last resize the screen of a
// full-screen program is sent again.
const resyncDelay = 150 * time.Millisecond

// resyncScreen sends the windows the screen of a full-screen program as
// the block has it. A window resizes its terminal as it draws, and the
// block as the request reaches it, while the program's output is on its
// way: what the program drew for one size may land in a screen of
// another, which full-screen programs, redrawing only what changed, do
// not mend. Shells' scrollback reflows alike in both and needs nothing.
func (b *block) resyncScreen() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.exited || len(b.clients) == 0 {
		return
	}
	snap := b.vt.Snapshot()
	if !bytes.Contains(snap, []byte("\x1b[?1049h")) {
		return
	}
	// Clear the alternate screen the window shows, then draw it anew.
	msg := append([]byte("\x1b[?1049h\x1b[H\x1b[2J"), snap...)
	for a := range b.clients {
		if !a.send(msg) {
			a.finish()
			delete(b.clients, a)
		}
	}
}

// kill hangs up the block and waits a little for it to end, then frees
// its screen.
func (b *block) kill() {
	b.mu.Lock()
	exited := b.exited
	b.mu.Unlock()
	if !exited {
		b.p.hangup()
	}
	select {
	case <-b.done:
	case <-time.After(time.Second):
		b.p.kill()
	}
	if b.keepOpen {
		b.vtIn.Close()
		b.vt.Close()
	}
}

// capture returns the block's screen and scrollback: its text, or, as
// "vt", the escape sequences that draw it. The text has a line for each
// row, unless unwrap joins the rows of a line the terminal wrapped.
func (b *block) capture(format string, trim, unwrap bool) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out string
	switch format {
	case "", "text":
		out = b.vt.Text()
		if !unwrap {
			cols, _ := b.vt.Size()
			out = splitRows(out, cols)
		}
	case "vt":
		out = string(b.vt.Snapshot())
	default:
		return "", fmt.Errorf("unknown format %q: text or vt", format)
	}
	if trim {
		lines := strings.Split(out, "\n")
		for i := range lines {
			lines[i] = strings.TrimRight(lines[i], " ")
		}
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
		out = strings.Join(lines, "\n")
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return out, nil
}

// splitRows breaks text whose wrapped lines the emulator joined into the
// rows it wrapped them in, cols cells wide. The emulator's text joins
// them always: a line wider than the screen is one it wrapped, and it
// wrapped each row as it filled, a wide character that would not fit
// going on to the next row.
func splitRows(text string, cols int) string {
	if cols <= 0 {
		return text
	}
	var b strings.Builder
	for i, line := range strings.Split(text, "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		w := 0
		for _, r := range line {
			rw := cellWidth(r)
			if w > 0 && w+rw > cols {
				b.WriteByte('\n')
				w = 0
			}
			b.WriteRune(r)
			w += rw
		}
	}
	return b.String()
}

// cellWidth is how many cells a character takes: none for a mark that
// joins the one before, two for a wide one.
func cellWidth(r rune) int {
	switch {
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	}
	return 1
}

// info describes the block and what runs in its foreground.
func (b *block) info() BlockInfo {
	b.mu.Lock()
	in := BlockInfo{
		ID: b.id, Shell: b.shell, Created: b.created, KeepOpen: b.keepOpen,
		Title: b.vt.Title(), LastOutput: b.lastOutput, LastInput: b.lastInput,
		Output: b.output, Bells: int(b.bells.Load()), Exited: b.exited, ExitCode: b.code,
		Attached: len(b.clients), Cols: b.cols, Rows: b.rows, Status: b.status.list(),
	}
	b.mu.Unlock()
	in.PID = b.p.pid()
	if in.Exited {
		in.Program = b.shell
		return in
	}
	fg := b.p.foreground()
	if fg <= 0 {
		fg = in.PID
	}
	in.Idle = fg == in.PID && len(b.command) == 0
	p := inspect(fg)
	in.Program, in.Args, in.Dir = p.programName(), p.args, p.dir
	if in.Program == "" {
		in.Program = b.shell
	}
	if in.Dir == "" && fg != in.PID {
		in.Dir = inspect(in.PID).dir
	}
	return in
}

// discard is the Conn of a block's emulator: it reads from a pipe that
// closes with the block, and drops what it writes.
type discard struct{ r *io.PipeReader }

func (d discard) Read(p []byte) (int, error)  { return d.r.Read(p) }
func (d discard) Write(p []byte) (int, error) { return len(p), nil }
func (d discard) Close() error                { return d.r.Close() }
