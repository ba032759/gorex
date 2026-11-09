// Package rex is GoRex's session server and its client.
//
// The server owns the pseudo-terminals: shells keep running when the app
// quits, and the next launch attaches to them again, with what they printed
// replayed. The app talks to it over a Unix socket: a control connection of
// JSON lines (requests and their responses), and one connection per
// attached session that carries the terminal's bytes both ways.
package rex

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"time"
)

// ProtocolVersion changes whenever the wire protocol does.
const ProtocolVersion = 2

// Version is GoRex's version, as "rex version" prints it.
const Version = "0.2.0"

// Request is a line of the control connection, from the client.
type Request struct {
	ID     int64          `json:"id"`
	Op     string         `json:"op"`
	SID    string         `json:"sid,omitempty"`
	Cols   int            `json:"cols,omitempty"`
	Rows   int            `json:"rows,omitempty"`
	Create *CreateOptions `json:"create,omitempty"`
	// Args are the arguments of the model's actions and of the requests
	// added after the first version.
	Args *Args `json:"args,omitempty"`
}

// Response answers the Request with the same ID.
type Response struct {
	ID    int64  `json:"id"`
	Error string `json:"error,omitempty"`
	// Code classifies an error: "not_found" or "ambiguous" for a target,
	// "usage" for a request that makes no sense, "no_client" for a client
	// action no app takes.
	Code string          `json:"code,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
	// Event is an event the connection subscribed to, in a line of its
	// own, whose ID is 0.
	Event *Event `json:"event,omitempty"`
}

// Event is something that changed, as "rex events" prints it: a server
// event, as "session_created", or a session's, as "session_event" whose
// payload holds the session's ID and the event.
type Event struct {
	Type      string         `json:"type"`
	Time      time.Time      `json:"time"`
	SessionID string         `json:"session_id,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	// Model is the model after the change, for a client that asked for it.
	Model *Model `json:"model,omitempty"`
}

// Args are what a request acts on, and how. Session, Window and Block are
// targets: IDs, labels, positions or the ends of IDs; CtxSession and
// CtxBlock are where the client runs (REX_SESSION and REX_BLOCK), the
// defaults of a request that names neither.
type Args struct {
	Session    string `json:"session,omitempty"`
	Window     string `json:"window,omitempty"`
	Block      string `json:"block,omitempty"`
	CtxSession string `json:"ctx_session,omitempty"`
	CtxBlock   string `json:"ctx_block,omitempty"`
	// Target is a second target, or one of any kind: the anchor of a
	// move, the other block of a swap, what "focus" goes to.
	Target string `json:"target,omitempty"`
	Split  string `json:"split,omitempty"`

	Label     string  `json:"label,omitempty"`
	Direction string  `json:"direction,omitempty"`
	Ratio     float32 `json:"ratio,omitempty"` // the new or moved block's share, 0 to 1
	Amount    float32 `json:"amount,omitempty"`
	Index     int     `json:"index,omitempty"`
	Before    string  `json:"before,omitempty"`
	After     string  `json:"after,omitempty"`
	NoFocus   bool    `json:"no_focus,omitempty"`
	Wrap      bool    `json:"wrap,omitempty"`
	Zoom      string  `json:"zoom,omitempty"` // "on", "off" or "" to toggle
	// Window label of a new session's first window.
	WindowLabel string         `json:"window_label,omitempty"`
	Create      *CreateOptions `json:"create,omitempty"`

	Data   string   `json:"data,omitempty"`
	Keys   []string `json:"keys,omitempty"`
	Format string   `json:"format,omitempty"`
	Trim   bool     `json:"trim,omitempty"`
	// Unwrap, on capture, joins the rows of a line the terminal wrapped.
	Unwrap bool `json:"unwrap,omitempty"`

	// Subscriptions: one session's events, every event, the model.
	All   bool `json:"all,omitempty"`
	Model bool `json:"model,omitempty"`

	// Client actions: the client, the action and its arguments, and the
	// answer of the app that ran one.
	Client   string          `json:"client,omitempty"`
	Action   string          `json:"action,omitempty"`
	ActionID int64           `json:"action_id,omitempty"`
	Params   json.RawMessage `json:"params,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
	Error    string          `json:"error,omitempty"`

	// Hello: who the client is.
	Kind string `json:"kind,omitempty"`
	Name string `json:"name,omitempty"`

	// Forget, on shutdown, drops the saved state, so that the next server
	// starts with nothing.
	Forget bool `json:"forget,omitempty"`
	// WantModel asks for the model after an action, with its result.
	WantModel bool `json:"want_model,omitempty"`
}

// ActResult is the answer to an action asked WantModel.
type ActResult struct {
	Result json.RawMessage `json:"result,omitempty"`
	Model  *Model          `json:"model,omitempty"`
}

// CreateOptions start a session.
type CreateOptions struct {
	// Command runs instead of the user's login shell.
	Command []string `json:"command,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Env     []string `json:"env,omitempty"`
	Cols    int      `json:"cols,omitempty"`
	Rows    int      `json:"rows,omitempty"`
	// Shell runs Command through that shell's login environment, the
	// user's shell when empty; "none" starts it directly. Interactive
	// also reads the shell's interactive startup files.
	Shell       string `json:"shell,omitempty"`
	Interactive bool   `json:"interactive,omitempty"`
	// KeepOpen keeps the block once its program exits.
	KeepOpen bool   `json:"keep_open,omitempty"`
	Label    string `json:"label,omitempty"`
}

// BlockInfo describes a block and what runs in it now.
type BlockInfo struct {
	ID       string    `json:"id"`
	Label    string    `json:"label,omitempty"`
	Session  string    `json:"session_id,omitempty"`
	Window   string    `json:"window_id,omitempty"`
	KeepOpen bool      `json:"keep_open,omitempty"`
	PID      int       `json:"pid"`
	Shell    string    `json:"shell"`
	Created  time.Time `json:"created"`
	// Program is the name of the program in the foreground, and Args its
	// arguments; Idle tells that it is the shell itself, at its prompt.
	Program string   `json:"program"`
	Args    []string `json:"args,omitempty"`
	Idle    bool     `json:"idle"`
	// Dir is the working directory of the program in the foreground.
	Dir string `json:"dir"`
	// Title is the last title a program set (OSC 0, OSC 2).
	Title      string    `json:"title,omitempty"`
	LastOutput time.Time `json:"lastOutput"`
	LastInput  time.Time `json:"lastInput"`
	Output     uint64    `json:"output"`
	Bells      int       `json:"bells"`
	Exited     bool      `json:"exited"`
	ExitCode   int       `json:"exitCode"`
	Attached   int       `json:"attached"`
	// Status is what programs reported of themselves (OSC 7501).
	Status []StatusRecord `json:"status,omitempty"`
	Cols   int            `json:"cols"`
	Rows   int            `json:"rows"`
}

// Hello is the server's answer to "hello".
type Hello struct {
	Version int       `json:"version"`
	PID     int       `json:"pid"`
	Started time.Time `json:"started"`
	Host    HostInfo  `json:"host"`
	// Exe is the server's executable, and ExeTime when it was modified
	// last as the server started: a development build compares them with
	// its own, to replace a server of an older build.
	Exe     string    `json:"exe,omitempty"`
	ExeTime time.Time `json:"exeTime,omitzero"`
	// ClientID is the ID the server gave the connection.
	ClientID string `json:"clientID,omitempty"`
	// Bin is the directory of the rex command the server puts first in
	// its terminals' PATH.
	Bin string `json:"bin,omitempty"`
}

// ClientInfo describes a client connected to the server.
type ClientInfo struct {
	ID        string    `json:"client_id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name,omitempty"`
	PID       int       `json:"pid,omitempty"`
	Connected time.Time `json:"connected_at"`
	Viewing   string    `json:"viewing_session_id,omitempty"`
}

// HostInfo describes the machine the server runs on.
type HostInfo struct {
	Name   string `json:"name"`
	Model  string `json:"model"`
	Chip   string `json:"chip"`
	Memory uint64 `json:"memory"`
	OS     string `json:"os"`
	User   string `json:"user"`
	Home   string `json:"home"`
	// Portable tells a machine with a battery, as a laptop, where the
	// model does not.
	Portable bool `json:"portable,omitempty"`
}

// Attach is the first line of a connection attaching to a session.
type Attach struct {
	Op   string `json:"op"` // "attach"
	SID  string `json:"sid"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// Dir returns the directory of the server's socket, log and state.
func Dir() string {
	if d := os.Getenv("GOREX_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "GoRex")
}

// SocketPath returns the path of the server's socket: in Dir, unless that
// is too long a path for a socket, then in the temporary directory.
func SocketPath() string {
	p := filepath.Join(Dir(), "server.sock")
	if len(p) < 100 {
		return p
	}
	h := fnv.New32a()
	h.Write([]byte(p))
	return filepath.Join(os.TempDir(), fmt.Sprintf("gorex-%d-%x.sock", os.Getuid(), h.Sum32()))
}

// BinDir returns the directory of the rex command the server gives its
// terminals.
func BinDir() string { return filepath.Join(Dir(), "bin") }
