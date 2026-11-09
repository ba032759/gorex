package rex

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Server owns the blocks, and the model of sessions, windows and layouts
// they are in.
type Server struct {
	mu     sync.Mutex
	model  Model
	blocks map[string]*block
	// tombs keep the exit codes of blocks that closed, a while, for
	// "rex wait" asked after the block went.
	tombs map[string]tomb
	conns map[*conn]struct{}

	started  time.Time
	host     HostInfo
	hostDone chan struct{}
	exe      string
	exeTime  time.Time
	idleFrom time.Time
	quit     chan struct{}
	quitOnce sync.Once
	forget   bool
	stopping bool
	// closed refuses new clients: the server stops, as it was idle.
	closed bool

	saveTimer *time.Timer
	lastSaved []byte

	// actions are the client actions sent to apps, waiting for their
	// answers.
	actions    map[int64]chan Args
	nextAction int64

	// last is what the watcher saw of each block, to tell what changed.
	last map[string]BlockInfo
}

type tomb struct {
	code int
	at   time.Time
}

// conn is a control connection, and the client at its other end.
type conn struct {
	id        string
	kind      string
	name      string
	pid       int // the client's process, for the wide events; 0 if unknown
	connected time.Time
	// viewing is the session an app shows, and active when it last told.
	viewing string
	active  time.Time

	out  chan []byte
	once sync.Once
	done chan struct{}
	sub  *subscription
}

// subscription is what a connection asked to hear.
type subscription struct {
	session string // one session's events, with its terminals'
	all     bool
	model   bool
	dirty   chan struct{}
}

// send queues a line; replies wait for room, events do not.
func (c *conn) send(b []byte, wait bool) {
	select {
	case <-c.done:
		return
	default:
	}
	if wait {
		select {
		case c.out <- b:
		case <-c.done:
		}
		return
	}
	select {
	case c.out <- b:
	default: // too far behind: an event is lost, the model is not
	}
}

func (c *conn) close() { c.once.Do(func() { close(c.done) }) }

// idleTimeout is how long a server with no sessions and no clients waits
// before it exits.
const idleTimeout = 10 * time.Minute

// tombTime is how long the exit code of a closed block is kept.
const tombTime = 10 * time.Minute

// Serve runs the server until it is shut down: it returns an error at
// once when another server already runs.
func Serve() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, "server.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := lockFile(lock); err != nil {
		return errors.New("rex: a server already runs")
	}
	defer lock.Close()
	sock := ListenPath()
	os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	os.Chmod(sock, 0o600)
	s := &Server{
		blocks: map[string]*block{}, tombs: map[string]tomb{}, conns: map[*conn]struct{}{},
		actions: map[int64]chan Args{}, last: map[string]BlockInfo{},
		started:  time.Now(),
		hostDone: make(chan struct{}), idleFrom: time.Now(), quit: make(chan struct{}),
	}
	s.exe, s.exeTime = Executable()
	linkBin(s.exe)
	go func() {
		s.host = hostInfo()
		close(s.hostDone)
	}()
	s.restore()
	go s.watchIdle()
	go s.watch()
	go func() {
		<-s.quit
		ln.Close()
	}()
	log.Printf("rex: serving on %s (pid %d)", sock, os.Getpid())
	s.mu.Lock()
	Wide("server", W{"event": "start", "exe": s.exe, "sessions": len(s.model.Sessions), "blocks": len(s.blocks)})
	s.mu.Unlock()
	for {
		c, err := ln.Accept()
		if err != nil {
			select {
			case <-s.quit:
				s.stop()
				os.Remove(sock)
				return nil
			default:
			}
			if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() {
				continue
			}
			return err
		}
		go s.handle(c)
	}
}

// stop saves the state, unless told to forget it, and ends every block.
func (s *Server) stop() {
	s.mu.Lock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	forget := s.forget
	all := make([]*block, 0, len(s.blocks))
	for _, b := range s.blocks {
		all = append(all, b)
	}
	s.mu.Unlock()
	if forget {
		os.Remove(statePath())
	} else {
		s.save()
	}
	// The blocks end as the server does, and must not take themselves out
	// of the state it just saved.
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	var wg sync.WaitGroup
	for _, b := range all {
		wg.Add(1)
		go func() {
			b.kill()
			wg.Done()
		}()
	}
	wg.Wait()
	Wide("server", W{"event": "stop", "forget": forget, "blocks": len(all),
		"uptime_s": time.Since(s.started).Seconds()})
	WideFlush()
}

// linkBin makes Dir/bin/rex the server's executable, for the PATH of its
// terminals: the rex command that matches the server comes first.
func linkBin(exe string) {
	if exe == "" {
		return
	}
	bin := BinDir()
	os.MkdirAll(bin, 0o700)
	link := filepath.Join(bin, "rex")
	if cur, err := os.Readlink(link); err == nil && cur == exe {
		return
	}
	os.Remove(link)
	if err := os.Symlink(exe, link); err != nil {
		log.Printf("rex: linking %s: %v", link, err)
	}
}

func (s *Server) watchIdle() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.quit:
			return
		case <-t.C:
		}
		s.mu.Lock()
		idle := len(s.blocks) == 0 && len(s.conns) == 0 && time.Since(s.idleFrom) > idleTimeout
		for id, t := range s.tombs {
			if time.Since(t.at) > tombTime {
				delete(s.tombs, id)
			}
		}
		s.mu.Unlock()
		if idle {
			log.Print("rex: idle, exiting")
			s.shutdown()
			return
		}
	}
}

func (s *Server) shutdown() { s.quitOnce.Do(func() { close(s.quit) }) }

func (s *Server) handle(c net.Conn) {
	r := bufio.NewReaderSize(c, 64<<10)
	first, err := r.ReadBytes('\n')
	if err != nil {
		c.Close()
		return
	}
	var a Attach
	if json.Unmarshal(first, &a) == nil && a.Op == "attach" {
		s.mu.Lock()
		b := s.blocks[a.SID]
		s.mu.Unlock()
		if b == nil {
			c.Write([]byte(`{"error":"no such block"}` + "\n"))
			c.Close()
			return
		}
		c.Write([]byte(`{"ok":true}` + "\n"))
		// What the reader buffered past the first line is input.
		if n := r.Buffered(); n > 0 {
			p, _ := r.Peek(n)
			b.input(p)
		}
		b.attach(c, a.Cols, a.Rows)
		return
	}
	s.control(c, r, first)
}

// control serves a control connection: a request per line.
func (s *Server) control(nc net.Conn, r *bufio.Reader, first []byte) {
	c := &conn{
		id: newID("client"), kind: "cli", connected: time.Now(), pid: peerPID(nc),
		out: make(chan []byte, 1024), done: make(chan struct{}),
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		nc.Close()
		return
	}
	s.conns[c] = struct{}{}
	s.mu.Unlock()
	go func() {
		for {
			select {
			case b := <-c.out:
				if _, err := nc.Write(b); err != nil {
					c.close()
					nc.Close()
					return
				}
			case <-c.done:
				return
			}
		}
	}()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.idleFrom = time.Now()
		left := c.kind != "cli"
		s.mu.Unlock()
		c.close()
		nc.Close()
		if left {
			s.emit(Event{Type: "client_left", Payload: map[string]any{"client_id": c.id}})
		}
	}()
	reply := func(res Response) {
		b, _ := json.Marshal(res)
		c.send(append(b, '\n'), true)
	}
	line := first
	for {
		var req Request
		if err := json.Unmarshal(line, &req); err == nil {
			switch req.Op {
			// Requests that wait, for a block to end or an app to
			// answer, are answered aside.
			case "kill", "wait", "client.do":
				go func() { reply(s.request(c, req)) }()
			default:
				reply(s.request(c, req))
			}
		}
		var err error
		line, err = r.ReadBytes('\n')
		if err != nil {
			return
		}
	}
}

func (s *Server) request(c *conn, req Request) Response {
	if req.Args == nil {
		req.Args = &Args{}
	}
	start := time.Now()
	data, err := s.do(c, req)
	res := Response{ID: req.ID}
	defer func() { s.wideRequest(c, req, res, start, len(res.Data)) }()
	if err != nil {
		res.Error = err.Error()
		var te *TargetError
		var ue usageError
		switch {
		case errors.As(err, &te):
			res.Code = "not_found"
			if len(te.Matches) > 1 {
				res.Code = "ambiguous"
			}
		case errors.As(err, &ue):
			res.Code = "usage"
		case errors.Is(err, ErrNoClient):
			res.Code = "no_client"
		case errors.Is(err, ErrServerGone):
			res.Code = "server_gone"
		case errors.Is(err, ErrNoTarget):
			res.Code = "not_found"
		}
	} else if data != nil {
		res.Data, _ = json.Marshal(data)
	}
	return res
}

// usageError is a request that makes no sense, as a move of a block next
// to itself.
type usageError string

func (e usageError) Error() string { return string(e) }

// ErrNoClient is a client action with no app to run it.
var ErrNoClient = errors.New("no app is connected: open GoRex")

func (s *Server) do(c *conn, req Request) (any, error) {
	a := req.Args
	switch req.Op {
	case "hello":
		select {
		case <-s.hostDone:
		case <-time.After(3 * time.Second):
		}
		s.mu.Lock()
		if a.Kind != "" {
			c.kind, c.name = a.Kind, a.Name
		}
		kind := c.kind
		s.mu.Unlock()
		if kind != "cli" {
			s.emit(Event{Type: "client_joined", Payload: map[string]any{"client": s.clientInfo(c)}})
		}
		return Hello{Version: ProtocolVersion, PID: os.Getpid(), Started: s.started, Host: s.host,
			Exe: s.exe, ExeTime: s.exeTime, ClientID: c.id, Bin: BinDir()}, nil
	case "list":
		s.mu.Lock()
		all := s.orderedBlocks()
		where := map[string][2]string{}
		for _, ss := range s.model.Sessions {
			for _, w := range ss.Windows {
				for _, id := range w.Layout.Blocks() {
					where[id] = [2]string{ss.ID, w.ID}
				}
			}
		}
		labels := map[*block]string{}
		for _, b := range all {
			labels[b] = b.label
		}
		s.mu.Unlock()
		infos := make([]BlockInfo, len(all))
		for i, b := range all {
			infos[i] = b.info()
			infos[i].Label = labels[b]
			infos[i].Session, infos[i].Window = where[b.id][0], where[b.id][1]
		}
		return infos, nil
	case "model":
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.snapshot(), nil
	case "client.inspect":
		s.mu.Lock()
		defer s.mu.Unlock()
		name := ""
		if a != nil {
			name = a.Client
		}
		c, err := s.appLocked(name)
		if err != nil {
			return nil, err
		}
		return s.clientInfoLocked(c), nil
	case "clients":
		s.mu.Lock()
		defer s.mu.Unlock()
		var out []ClientInfo
		for cc := range s.conns {
			out = append(out, s.clientInfoLocked(cc))
		}
		slices.SortFunc(out, func(a, b ClientInfo) int { return a.Connected.Compare(b.Connected) })
		return out, nil
	case "create":
		// A block in a session of its own: the first version's request.
		o := CreateOptions{}
		if req.Create != nil {
			o = *req.Create
		}
		res, err := s.act("session.new", &Args{Create: &o})
		if err != nil {
			return nil, err
		}
		r := res.(map[string]any)
		id := r["block_id"].(string)
		s.mu.Lock()
		b := s.blocks[id]
		s.mu.Unlock()
		in := b.info()
		in.Session = r["session_id"].(string)
		in.Window = r["window_id"].(string)
		return in, nil
	case "kill":
		_, err := s.act("block.close", &Args{Block: req.SID})
		var te *TargetError
		if errors.As(err, &te) {
			return nil, nil // already gone
		}
		return nil, err
	case "resize":
		s.mu.Lock()
		b := s.blocks[req.SID]
		s.mu.Unlock()
		if b == nil {
			return nil, &TargetError{Kind: "block", Target: req.SID}
		}
		b.resize(req.Cols, req.Rows)
		return nil, nil
	case "shutdown", "shutdown.idle":
		s.mu.Lock()
		if req.Op == "shutdown.idle" {
			// Idle: no session and no client but this one. A client
			// that connects from now on is refused.
			if n, others := len(s.model.Sessions), len(s.conns)-1; n > 0 || others > 0 {
				s.mu.Unlock()
				return nil, fmt.Errorf("the server is in use: %d sessions, %d other clients", n, others)
			}
			s.closed = true
		}
		s.forget = a.Forget
		s.mu.Unlock()
		go func() {
			time.Sleep(50 * time.Millisecond)
			s.shutdown()
		}()
		return nil, nil
	case "view":
		s.mu.Lock()
		c.viewing, c.active = a.Session, time.Now()
		s.mu.Unlock()
		return nil, nil
	case "subscribe":
		sub := &subscription{all: a.All, model: a.Model, dirty: make(chan struct{}, 1)}
		if a.Session != "" {
			s.mu.Lock()
			ss, err := s.resolveSession(a.Session)
			s.mu.Unlock()
			if err != nil {
				return nil, err
			}
			sub.session = ss.ID
		}
		s.mu.Lock()
		c.sub = sub
		s.mu.Unlock()
		if sub.model {
			go s.pushModel(c, sub)
			sub.dirty <- struct{}{}
		}
		return nil, nil
	case "capture":
		b, err := s.targetBlock(a)
		if err != nil {
			return nil, err
		}
		text, err := b.capture(a.Format, a.Trim, a.Unwrap)
		if err != nil {
			return nil, usageError(err.Error())
		}
		return map[string]any{"block_id": b.id, "text": text}, nil
	case "send":
		b, err := s.targetBlock(a)
		if err != nil {
			return nil, err
		}
		if err := b.input([]byte(a.Data)); err != nil {
			return nil, err
		}
		return map[string]any{"block_id": b.id}, nil
	case "sendKey":
		b, err := s.targetBlock(a)
		if err != nil {
			return nil, err
		}
		b.mu.Lock()
		app := b.appCursor
		b.mu.Unlock()
		var data []byte
		for _, k := range a.Keys {
			p, err := EncodeKey(k, app)
			if err != nil {
				return nil, usageError(err.Error())
			}
			data = append(data, p...)
		}
		if err := b.input(data); err != nil {
			return nil, err
		}
		return map[string]any{"block_id": b.id}, nil
	case "status.report":
		// A program's status, from a hook rather than its terminal.
		b, err := s.targetBlock(a)
		if err != nil {
			return nil, err
		}
		if err := b.report(a.Data); err != nil {
			return nil, usageError(err.Error())
		}
		return map[string]any{"block_id": b.id}, nil
	case "status.list":
		b, err := s.targetBlock(a)
		if err != nil {
			return nil, err
		}
		b.mu.Lock()
		list := b.status.list()
		b.mu.Unlock()
		return map[string]any{"block_id": b.id, "records": list}, nil
	case "wait":
		return s.wait(a)
	case "client.do":
		return s.clientDo(a)
	case "client.reply":
		s.mu.Lock()
		ch := s.actions[a.ActionID]
		delete(s.actions, a.ActionID)
		s.mu.Unlock()
		if ch != nil {
			ch <- *a
		}
		return nil, nil
	}
	res, err := s.act(req.Op, a)
	if errors.Is(err, errUnknownAction) {
		return nil, fmt.Errorf("unknown op %q", req.Op)
	}
	if err != nil {
		return nil, err
	}
	if a.WantModel {
		b, _ := json.Marshal(res)
		s.mu.Lock()
		m := s.snapshot()
		s.mu.Unlock()
		return ActResult{Result: b, Model: m}, nil
	}
	return res, nil
}

func (s *Server) clientInfo(c *conn) ClientInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clientInfoLocked(c)
}

func (s *Server) clientInfoLocked(c *conn) ClientInfo {
	return ClientInfo{ID: c.id, Kind: c.kind, Name: c.name, Connected: c.connected, Viewing: c.viewing}
}

// orderedBlocks returns the blocks in the order of the sessions, their
// windows and layouts, then any other. The lock is held.
func (s *Server) orderedBlocks() []*block {
	var out []*block
	seen := map[string]bool{}
	for _, ss := range s.model.Sessions {
		for _, w := range ss.Windows {
			for _, id := range w.Layout.Blocks() {
				if b := s.blocks[id]; b != nil && !seen[id] {
					out = append(out, b)
					seen[id] = true
				}
			}
		}
	}
	var rest []*block
	for id, b := range s.blocks {
		if !seen[id] {
			rest = append(rest, b)
		}
	}
	slices.SortFunc(rest, func(a, b *block) int { return a.created.Compare(b.created) })
	return append(out, rest...)
}

// snapshot returns a copy of the model, with what it says of each block.
// The lock is held.
func (s *Server) snapshot() *Model {
	m := s.model.Clone()
	m.Blocks = map[string]*BlockMeta{}
	for _, ss := range s.model.Sessions {
		for _, w := range ss.Windows {
			for _, id := range w.Layout.Blocks() {
				b := s.blocks[id]
				if b == nil {
					continue
				}
				b.mu.Lock()
				m.Blocks[id] = &BlockMeta{
					ID: id, Label: b.label, Session: ss.ID, Window: w.ID, Shell: b.shell,
					Command: slices.Clone(b.command), PID: b.p.pid(), Cols: b.cols, Rows: b.rows,
					KeepOpen: b.keepOpen, Exited: b.exited, ExitCode: b.code, Created: b.created,
					Restored: b.restored,
				}
				b.mu.Unlock()
			}
		}
	}
	return m
}

// pushModel sends the model to a client that asked for it, as it
// changes: the latest, however many changes it missed.
func (s *Server) pushModel(c *conn, sub *subscription) {
	sent := uint64(0)
	first := true
	for {
		select {
		case <-c.done:
			return
		case <-sub.dirty:
		}
		s.mu.Lock()
		if !first && s.model.Revision == sent {
			s.mu.Unlock()
			continue
		}
		m := s.snapshot()
		s.mu.Unlock()
		sent, first = m.Revision, false
		b, _ := json.Marshal(Response{Event: &Event{Type: "model", Time: time.Now(), Model: m}})
		c.send(append(b, '\n'), true)
	}
}

// changed notes a change of the model: its revision goes up, the clients
// that mirror it hear of it, and it is saved soon. The lock is held.
func (s *Server) changed(sessions ...string) {
	s.model.Revision++
	for c := range s.conns {
		if c.sub != nil && c.sub.model {
			select {
			case c.sub.dirty <- struct{}{}:
			default:
			}
		}
	}
	for _, id := range sessions {
		if id != "" {
			s.emitLocked(sessionEvent(id, "session_view_changed", map[string]any{"revision": s.model.Revision}))
		}
	}
	if s.saveTimer == nil {
		s.saveTimer = time.AfterFunc(300*time.Millisecond, s.save)
	} else {
		s.saveTimer.Reset(300 * time.Millisecond)
	}
}

// sessionEvent wraps an event of a session as Rex's server sends it.
func sessionEvent(session, typ string, payload map[string]any) Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return Event{Type: "session_event", SessionID: session, Payload: map[string]any{
		"session_id": session,
		"event":      map[string]any{"type": typ, "payload": payload},
	}}
}

// blockEvent is an event a terminal reports, in its session.
func blockEvent(session, blockID, name string, payload map[string]any) Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return sessionEvent(session, "block_event", map[string]any{
		"block_id": blockID, "creator_name": "com.superlogical.terminal", "name": name, "payload": payload,
	})
}

// emit sends an event to the clients that subscribed to it.
func (s *Server) emit(ev Event) {
	s.mu.Lock()
	s.emitLocked(ev)
	s.mu.Unlock()
}

func (s *Server) emitLocked(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	wideEvent(ev)
	var b []byte
	for c := range s.conns {
		sub := c.sub
		if sub == nil || !wants(sub, ev) {
			continue
		}
		if b == nil {
			b, _ = json.Marshal(Response{Event: &ev})
			b = append(b, '\n')
		}
		c.send(b, false)
	}
}

// wants tells the events a subscription hears: every one with all; one
// session's, its terminals' too; else the server's, and the sessions'
// but for their terminals'. A client mirroring the model hears none: the
// model tells it what changed.
func wants(sub *subscription, ev Event) bool {
	if sub.all {
		return true
	}
	if sub.session != "" {
		return ev.SessionID == sub.session
	}
	if sub.model {
		return false
	}
	switch ev.Type {
	case "client_joined", "client_left", "client_changed":
		return false
	case "session_event":
		e, _ := ev.Payload["event"].(map[string]any)
		return e["type"] != "block_event"
	}
	return true
}

// The saved state: the model, and what it takes to start each block again
// where it was.
type savedState struct {
	Version  int                   `json:"version"`
	Sessions []*Session            `json:"sessions"`
	Blocks   map[string]savedBlock `json:"blocks"`
}

type savedBlock struct {
	Label    string   `json:"label,omitempty"`
	Dir      string   `json:"dir,omitempty"`
	Cols     int      `json:"cols,omitempty"`
	Rows     int      `json:"rows,omitempty"`
	Command  []string `json:"command,omitempty"`
	KeepOpen bool     `json:"keep_open,omitempty"`
}

func statePath() string { return filepath.Join(Dir(), "state.json") }

// DiscardState deletes the saved sessions, for a server that does not
// run: the next one starts with none.
func DiscardState() error {
	if err := os.Remove(statePath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// save writes the state, when it changed.
func (s *Server) save() {
	s.mu.Lock()
	st := savedState{Version: 1, Sessions: s.model.Clone().Sessions, Blocks: map[string]savedBlock{}}
	blocks := map[string]*block{}
	labels := map[string]string{}
	for id, b := range s.blocks {
		blocks[id], labels[id] = b, b.label
	}
	s.mu.Unlock()
	for id, b := range blocks {
		in := b.info()
		st.Blocks[id] = savedBlock{Label: labels[id], Dir: in.Dir, Cols: in.Cols, Rows: in.Rows,
			Command: b.command, KeepOpen: b.keepOpen}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	s.mu.Lock()
	same := string(data) == string(s.lastSaved)
	s.lastSaved = data
	s.mu.Unlock()
	if same {
		return
	}
	tmp := statePath() + ".tmp"
	err = os.WriteFile(tmp, data, 0o600)
	if err == nil {
		err = os.Rename(tmp, statePath())
	}
	if err != nil {
		log.Printf("rex: saving the state: %v", err)
	}
}

// restore starts the sessions of the saved state again: the blocks' shells
// in the directories they were in, as the programs that ran there ended
// with the server that ran them. The first version's layout of tabs
// becomes a session.
func (s *Server) restore() {
	var st savedState
	if b, err := os.ReadFile(statePath()); err == nil {
		if json.Unmarshal(b, &st) != nil {
			log.Printf("rex: %s is not a state; starting afresh", statePath())
			st = savedState{}
		}
	} else if b, err := os.ReadFile(filepath.Join(Dir(), "layout.json")); err == nil {
		st = migrateLayout(b)
		if len(st.Sessions) > 0 {
			if err := os.Rename(filepath.Join(Dir(), "layout.json"), filepath.Join(Dir(), "layout.json.migrated")); err != nil {
				log.Printf("rex: setting the old layout aside: %v", err)
			}
		}
	}
	if len(st.Sessions) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	home, _ := os.UserHomeDir()
	for _, ss := range st.Sessions {
		for _, w := range ss.Windows {
			for _, id := range w.Layout.Blocks() {
				sb := st.Blocks[id]
				dir := sb.Dir
				if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
					dir = home
				}
				// The shell alone starts again: a command could do what
				// it did once more, as a deploy.
				o := CreateOptions{Dir: dir, Cols: sb.Cols, Rows: sb.Rows, Label: sb.Label}
				b, err := s.startBlock(id, o, ss.ID)
				if err != nil {
					log.Printf("rex: restoring %s: %v", id, err)
					w.Layout = w.Layout.remove(id)
					continue
				}
				b.restored = true
			}
			w.Recent = slices.DeleteFunc(w.Recent, func(id string) bool { return s.blocks[id] == nil })
			if leaf, _ := w.Layout.find(w.Zoom); leaf == nil {
				w.Zoom = ""
			}
			if leaf, _ := w.Layout.find(w.Focus); leaf == nil {
				w.Focus = ""
				if bs := w.Layout.Blocks(); len(bs) > 0 {
					w.setFocus(bs[0])
				}
			}
		}
		ss.Windows = slices.DeleteFunc(ss.Windows, func(w *Window) bool { return w.Layout == nil })
		if len(ss.Windows) == 0 {
			continue
		}
		if ss.windowIndex(ss.Active) < 0 {
			ss.Active = ss.Windows[0].ID
		}
		s.model.Sessions = append(s.model.Sessions, ss)
	}
	s.model.Revision = 1
	log.Printf("rex: restored %d sessions", len(s.model.Sessions))
}

// migrateLayout turns the first version's layout of tabs into a session.
func migrateLayout(raw []byte) savedState {
	var old struct {
		Active int `json:"active"`
		Tabs   []struct {
			Name  string          `json:"name"`
			Root  json.RawMessage `json:"root"`
			Focus int             `json:"focus"`
			Zoom  bool            `json:"zoom"`
		} `json:"tabs"`
	}
	if json.Unmarshal(raw, &old) != nil || len(old.Tabs) == 0 {
		return savedState{}
	}
	type node struct {
		Dir      string  `json:"dir"`
		Cols     int     `json:"cols"`
		Rows     int     `json:"rows"`
		Vertical bool    `json:"vertical"`
		Ratio    float32 `json:"ratio"`
		A        *node   `json:"a"`
		B        *node   `json:"b"`
	}
	st := savedState{Version: 1, Blocks: map[string]savedBlock{}}
	ss := &Session{ID: newID("session"), Created: time.Now()}
	var conv func(n *node) *Layout
	conv = func(n *node) *Layout {
		if n.A == nil || n.B == nil {
			id := newID("block")
			st.Blocks[id] = savedBlock{Dir: n.Dir, Cols: n.Cols, Rows: n.Rows}
			return &Layout{Block: id}
		}
		return &Layout{Split: newID("split"), Vertical: n.Vertical, Ratio: n.Ratio, A: conv(n.A), B: conv(n.B)}
	}
	for i, t := range old.Tabs {
		var root node
		if json.Unmarshal(t.Root, &root) != nil {
			continue
		}
		w := &Window{ID: newID("window"), Label: t.Name, Layout: conv(&root)}
		bs := w.Layout.Blocks()
		w.setFocus(bs[min(max(t.Focus, 0), len(bs)-1)])
		if t.Zoom && len(bs) > 1 {
			w.Zoom = w.Focus
		}
		ss.Windows = append(ss.Windows, w)
		if i == old.Active {
			ss.Active = w.ID
		}
	}
	if len(ss.Windows) == 0 {
		return savedState{}
	}
	st.Sessions = []*Session{ss}
	return st
}

func newID(kind string) string {
	var b [6]byte
	rand.Read(b[:])
	return kind + ":" + hex.EncodeToString(b[:])
}

// Spawn starts a server in the background, as a process of its own that
// outlives the app: the executable run with -server.
func Spawn() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(), 0o700); err != nil {
		return err
	}
	logf, err := os.OpenFile(filepath.Join(Dir(), "server.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logf.Close()
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devnull.Close()
	cmd := exec.Command(exe, "-server")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devnull, logf, logf
	cmd.SysProcAttr = detached()
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
