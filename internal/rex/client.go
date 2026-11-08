package rex

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Client is a control connection to the server.
type Client struct {
	conn net.Conn

	wmu     sync.Mutex
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan Response
	err     error
	closed  chan struct{}

	// OnEvent, when set before Subscribe, hears the events the client
	// subscribed to, from the reading goroutine.
	OnEvent func(Event)
}

// Error is an error the server answered, with what kind it is: Code is
// "not_found" or "ambiguous" for a target, "usage", "no_client" or "".
type Error struct {
	Code, Msg string
}

func (e *Error) Error() string { return e.Msg }

// Connect connects to the server, starting one in the background when
// none runs.
func Connect() (*Client, error) {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		if err := Spawn(); err != nil {
			return nil, fmt.Errorf("rex: starting the server: %w", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			time.Sleep(30 * time.Millisecond)
			conn, err = net.Dial("unix", SocketPath())
			if err == nil {
				break
			}
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("rex: connecting to the server: %w", err)
			}
		}
	}
	c := &Client{conn: conn, pending: map[int64]chan Response{}, closed: make(chan struct{})}
	go c.read()
	return c, nil
}

// Dial connects to the server that runs, without starting one.
func Dial() (*Client, error) {
	conn, err := net.Dial("unix", SocketPath())
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, pending: map[int64]chan Response{}, closed: make(chan struct{})}
	go c.read()
	return c, nil
}

// Executable returns the path of the running executable and when it was
// modified last.
func Executable() (string, time.Time) {
	exe, err := os.Executable()
	if err != nil {
		return "", time.Time{}
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	st, err := os.Stat(exe)
	if err != nil {
		return exe, time.Time{}
	}
	return exe, st.ModTime()
}

// Restart shuts the server down, ending its sessions, waits for it to
// exit, and connects to a new one.
func (c *Client) Restart(pid int) (*Client, error) {
	c.Shutdown()
	c.Close()
	deadline := time.Now().Add(3 * time.Second)
	for pid > 0 && processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	return Connect()
}

// Closed is closed once the connection to the server is lost.
func (c *Client) Closed() <-chan struct{} { return c.closed }

func (c *Client) read() {
	r := bufio.NewReaderSize(c.conn, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			c.mu.Lock()
			c.err = err
			for id, ch := range c.pending {
				ch <- Response{ID: id, Error: "connection to the server lost"}
				delete(c.pending, id)
			}
			c.mu.Unlock()
			close(c.closed)
			return
		}
		var res Response
		if json.Unmarshal(line, &res) != nil {
			continue
		}
		if res.Event != nil {
			if f := c.OnEvent; f != nil {
				f(*res.Event)
			}
			continue
		}
		c.mu.Lock()
		ch := c.pending[res.ID]
		delete(c.pending, res.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- res
		}
	}
}

// call sends a request and decodes the response's data into out.
func (c *Client) call(req Request, out any) error { return c.callFor(req, out, 15*time.Second) }

// callFor is call waiting at most d for the answer, or forever when d is 0.
func (c *Client) callFor(req Request, out any, d time.Duration) error {
	ch := make(chan Response, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return errors.New("rex: connection to the server lost")
	}
	c.nextID++
	req.ID = c.nextID
	c.pending[req.ID] = ch
	c.mu.Unlock()
	b, _ := json.Marshal(req)
	c.wmu.Lock()
	_, err := c.conn.Write(append(b, '\n'))
	c.wmu.Unlock()
	if err != nil {
		return err
	}
	var res Response
	var timeout <-chan time.Time
	if d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		timeout = t.C
	}
	select {
	case res = <-ch:
	case <-timeout:
		c.mu.Lock()
		delete(c.pending, req.ID)
		c.mu.Unlock()
		return fmt.Errorf("rex: %s timed out", req.Op)
	}
	if res.Error != "" {
		return &Error{Code: res.Code, Msg: res.Error}
	}
	if out != nil && len(res.Data) > 0 {
		return json.Unmarshal(res.Data, out)
	}
	return nil
}

func (c *Client) Hello() (h Hello, err error) {
	err = c.call(Request{Op: "hello"}, &h)
	return
}

// HelloAs says who the client is: "app" for an app, which client actions
// go to, "cli" for the command line.
func (c *Client) HelloAs(kind, name string) (h Hello, err error) {
	err = c.call(Request{Op: "hello", Args: &Args{Kind: kind, Name: name}}, &h)
	return
}

func (c *Client) List() (s []BlockInfo, err error) {
	err = c.call(Request{Op: "list"}, &s)
	return
}

// Create starts a block in a session of its own.
func (c *Client) Create(o CreateOptions) (s BlockInfo, err error) {
	err = c.call(Request{Op: "create", Create: &o}, &s)
	return
}

// Model returns the model of every session.
func (c *Client) Model() (m *Model, err error) {
	err = c.call(Request{Op: "model"}, &m)
	return
}

// Clients lists the clients connected to the server.
func (c *Client) Clients() (cs []ClientInfo, err error) {
	err = c.call(Request{Op: "clients"}, &cs)
	return
}

// Do sends a request of the op with args, and decodes its answer into out.
func (c *Client) Do(op string, args *Args, out any) error {
	return c.call(Request{Op: op, Args: args}, out)
}

// DoFor is Do waiting at most d, or forever when d is 0: for wait and for
// client actions.
func (c *Client) DoFor(op string, args *Args, out any, d time.Duration) error {
	return c.callFor(Request{Op: op, Args: args}, out, d)
}

// Act performs an action of the model and returns its result and the
// model after it.
func (c *Client) Act(op string, args *Args) (json.RawMessage, *Model, error) {
	if args == nil {
		args = &Args{}
	}
	a := *args
	a.WantModel = true
	var r ActResult
	if err := c.call(Request{Op: op, Args: &a}, &r); err != nil {
		return nil, nil, err
	}
	return r.Result, r.Model, nil
}

// Subscribe asks for events, which OnEvent hears: a session's, every
// one, or the model as it changes.
func (c *Client) Subscribe(args Args) error {
	return c.call(Request{Op: "subscribe", Args: &args}, nil)
}

// View tells the server the session an app shows: the CLI's default.
func (c *Client) View(session string) error {
	return c.call(Request{Op: "view", Args: &Args{Session: session}}, nil)
}

// Reply answers a client action the server sent.
func (c *Client) Reply(id int64, result any, err error) error {
	a := &Args{ActionID: id}
	if err != nil {
		a.Error = err.Error()
	} else if result != nil {
		a.Result, _ = json.Marshal(result)
	}
	return c.call(Request{Op: "client.reply", Args: a}, nil)
}

func (c *Client) Kill(sid string) error { return c.call(Request{Op: "kill", SID: sid}, nil) }

func (c *Client) Resize(sid string, cols, rows int) error {
	return c.call(Request{Op: "resize", SID: sid, Cols: cols, Rows: rows}, nil)
}

// Shutdown ends the server and its blocks, which start again with the
// next server, in their directories.
func (c *Client) Shutdown() error { return c.call(Request{Op: "shutdown"}, nil) }

// ShutdownForget ends the server and its blocks, for good.
func (c *Client) ShutdownForget() error {
	return c.call(Request{Op: "shutdown", Args: &Args{Forget: true}}, nil)
}

func (c *Client) Close() error { return c.conn.Close() }

// Stream is a session attached to, for a terminal's Conn: it attaches
// once it knows the size of the screen, from the first Resize, or after
// a moment at the size it had last.
type Stream struct {
	c        *Client
	sid      string
	attachMu sync.Mutex // one attach at a time
	mu       sync.Mutex
	cond     *sync.Cond
	conn     net.Conn
	err      error
	closed   bool
	// cols and rows are the terminal's size; sentCols and sentRows the
	// size the server was told last.
	cols, rows         int
	sentCols, sentRows int

	// OnData, when set, is called with the size of what the session
	// prints, from the reading goroutine.
	OnData func(n int)
}

// Stream returns a stream of the session sid, which attaches lazily.
func (c *Client) Stream(sid string, cols, rows int) *Stream {
	s := &Stream{c: c, sid: sid, cols: cols, rows: rows}
	s.cond = sync.NewCond(&s.mu)
	time.AfterFunc(400*time.Millisecond, func() {
		s.attach()
		s.sync()
	})
	return s
}

// attach attaches to the session at the terminal's size, once.
func (s *Stream) attach() {
	s.attachMu.Lock()
	defer s.attachMu.Unlock()
	s.mu.Lock()
	if s.conn != nil || s.err != nil || s.closed {
		s.mu.Unlock()
		return
	}
	cols, rows := s.cols, s.rows
	s.mu.Unlock()
	conn, err := net.Dial("unix", SocketPath())
	if err == nil {
		b, _ := json.Marshal(Attach{Op: "attach", SID: s.sid, Cols: cols, Rows: rows})
		if _, err = conn.Write(append(b, '\n')); err == nil {
			err = readOK(conn)
		}
		if err != nil {
			conn.Close()
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		if err == nil {
			conn.Close()
		}
		return
	}
	if err != nil {
		s.err = err
	} else {
		s.conn, s.sentCols, s.sentRows = conn, cols, rows
	}
	s.cond.Broadcast()
}

// sync tells the server the terminal's size, once attached, when it was
// told another.
func (s *Stream) sync() error {
	s.mu.Lock()
	if s.conn == nil || s.cols == s.sentCols && s.rows == s.sentRows {
		s.mu.Unlock()
		return nil
	}
	cols, rows := s.cols, s.rows
	s.sentCols, s.sentRows = cols, rows
	s.mu.Unlock()
	return s.c.Resize(s.sid, cols, rows)
}

// readOK reads the server's answer to an attach, a byte at a time so as
// to read nothing past it.
func readOK(conn net.Conn) error {
	var line []byte
	b := make([]byte, 1)
	for {
		if _, err := conn.Read(b); err != nil {
			return err
		}
		if b[0] == '\n' {
			break
		}
		line = append(line, b[0])
		if len(line) > 4096 {
			return errors.New("rex: bad answer")
		}
	}
	var res struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(line, &res); err != nil {
		return err
	}
	if !res.OK {
		return errors.New(res.Error)
	}
	return nil
}

func (s *Stream) wait() (net.Conn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.conn == nil && s.err == nil && !s.closed {
		s.cond.Wait()
	}
	if s.closed {
		return nil, io.EOF
	}
	return s.conn, s.err
}

func (s *Stream) Read(p []byte) (int, error) {
	conn, err := s.wait()
	if err != nil {
		return 0, err
	}
	n, err := conn.Read(p)
	if n > 0 && s.OnData != nil {
		s.OnData(n)
	}
	return n, err
}

func (s *Stream) Write(p []byte) (int, error) {
	conn, err := s.wait()
	if err != nil {
		return 0, err
	}
	return conn.Write(p)
}

// Resize tells the session the size of the screen, attaching first.
func (s *Stream) Resize(cols, rows int) error {
	s.mu.Lock()
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	s.attach()
	return s.sync()
}

// Close detaches; the session goes on.
func (s *Stream) Close() error {
	s.mu.Lock()
	s.closed = true
	conn := s.conn
	s.cond.Broadcast()
	s.mu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// Size returns the size the session was last given.
func (s *Stream) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}
