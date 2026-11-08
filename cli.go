package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"gorex/internal/rex"
)

// The rex command, after Rex's: the same commands, flags, targets and exit
// statuses, on GoRex's server. Its context is the terminal it runs in, by
// REX_SESSION and REX_BLOCK.

// Exit statuses.
const (
	exitOK       = 0
	exitFailed   = 1
	exitUsage    = 2
	exitTarget   = 3
	exitNoServer = 4
)

// cliError is an error with the status the command exits with.
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }

// errQuiet ends the command with a status and nothing to say, as a wait
// for a program that exited with it.
var errQuiet = errors.New("")

// exitWith is the status of a program waited for.
func exitWith(code int) error { return &cliError{code % 256, errQuiet} }

func (e *cliError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return &cliError{exitUsage, fmt.Errorf(format, args...)}
}

// exitCode returns the status an error ends the command with.
func exitCode(err error) int {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	var re *rex.Error
	if errors.As(err, &re) {
		switch re.Code {
		case "not_found", "ambiguous":
			return exitTarget
		case "usage":
			return exitUsage
		}
	}
	return exitFailed
}

// cliCommands are the commands of rex, which run the CLI rather than the
// app when GoRex is started with one.
var cliCommands = []string{
	"ls", "new", "run", "split", "send", "send-key", "capture", "kill", "focus", "zoom", "move", "swap",
	"resize", "close", "session", "window", "block", "client", "events", "wait", "do", "actions", "keymap",
	"config", "skill", "server", "version", "completion", "help", "--help", "-h", "status",
	// GoRex's first commands.
	"list", "new-tab", "send-keys",
}

func isCLICommand(s string) bool { return slices.Contains(cliCommands, s) }

// global are the options every command takes.
type global struct {
	autostart bool
	timeout   time.Duration
	client    string
}

var opts = global{autostart: true, timeout: 10 * time.Second}

// runCLI runs rex with its arguments and returns the exit status.
func runCLI(args []string) int {
	// The global options may come before the command.
	for len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "--help" && args[0] != "-h" {
		name, val, hasVal := strings.Cut(args[0], "=")
		next := func() (string, bool) {
			if hasVal {
				return val, true
			}
			if len(args) > 1 {
				args = args[1:]
				return args[0], true
			}
			return "", false
		}
		switch name {
		case "-S", "--server":
			fatalf("connecting to other servers is not supported: GoRex's server runs on this machine")
			return exitUsage
		case "-C", "--client":
			v, _ := next()
			opts.client = v
		case "--timeout":
			v, _ := next()
			d, err := time.ParseDuration(v)
			if err != nil {
				fatalf("--timeout: %v", err)
				return exitUsage
			}
			opts.timeout = d
		case "--autostart":
			opts.autostart = !hasVal || val == "true"
		case "--color":
			next()
		case "-v", "--verbose", "-vv":
		default:
			fatalf("unknown option %s", args[0])
			return exitUsage
		}
		args = args[1:]
	}
	if len(args) == 0 {
		printHelp("")
		return exitOK
	}
	if c := os.Getenv("REX_CLIENT"); opts.client == "" {
		opts.client = c
	}
	start := time.Now()
	err := dispatch(args[0], args[1:])
	code := exitOK
	if err != nil {
		if !errors.Is(err, errQuiet) {
			fatalf("%v", err)
		}
		code = exitCode(err)
	}
	wideCommand(args, code, err, start)
	return code
}

// wideCommand records a rex command that ran: which, from where (a GoRex
// terminal, an agent), how it ended and how long it took. Of its
// arguments, it keeps the subcommands and the flags' names, not the
// values: those may be text typed into terminals.
func wideCommand(args []string, code int, err error, start time.Time) {
	switch args[0] {
	case "help", "--help", "-h", "completion", "__complete":
		return
	}
	var words, flags []string
	// The words before the first flag name the command; after a flag, a
	// word may be the flag's value.
	for _, a := range args {
		if a == "--" {
			break // the command it runs
		}
		switch {
		case strings.HasPrefix(a, "-"):
			name, _, _ := strings.Cut(a, "=")
			flags = append(flags, name)
		case len(flags) == 0 && len(words) < 4 && !strings.ContainsAny(a, " /=\n") && len(a) <= 24:
			words = append(words, a)
		}
	}
	w := rex.W{
		"command":     strings.Join(words, " "),
		"exit_code":   code,
		"ok":          code == exitOK,
		"duration_ms": float64(time.Since(start).Microseconds()) / 1000,
		"n_args":      len(args) - 1,
		"in_rex":      os.Getenv("REX_BLOCK") != "",
	}
	if len(flags) > 0 {
		w["flags"] = strings.Join(flags, ",")
	}
	if err != nil && !errors.Is(err, errQuiet) {
		w["error"] = err.Error()
	}
	for k, env := range map[string]string{
		"rex_session": "REX_SESSION", "rex_block": "REX_BLOCK", "client": "REX_CLIENT",
	} {
		if v := os.Getenv(env); v != "" {
			w[k] = v
		}
	}
	switch {
	case os.Getenv("CLAUDECODE") != "":
		w["agent"] = "claude-code"
	case os.Getenv("CODEX_SANDBOX") != "" || os.Getenv("CODEX_HOME") != "":
		w["agent"] = "codex"
	}
	rex.Wide("cli", w)
	rex.WideFlush()
}

func dispatch(cmd string, args []string) error {
	switch cmd {
	case "help", "--help", "-h":
		printHelp(strings.Join(args, " "))
		return nil
	case "version":
		return cliVersion(args)
	case "ls":
		return cliSessionLs(args)
	case "new":
		return cliSessionNew(args)
	case "run":
		return cliRun(args, false)
	case "split":
		return cliRun(args, true)
	case "send":
		return cliSend(args)
	case "send-key":
		return cliSendKey(args)
	case "send-keys":
		return cliSendKeysCompat(args)
	case "capture":
		return cliCapture(args)
	case "kill":
		return cliSessionKill(args)
	case "focus":
		return cliFocus(args)
	case "zoom":
		return cliZoom(args)
	case "move":
		return cliMove(args)
	case "swap":
		return cliSwap(args)
	case "resize":
		return cliResize(args)
	case "close":
		return cliBlockClose(args)
	case "list":
		return cliTree(args)
	case "new-tab":
		return cliWindowNew(args)
	case "session":
		return sub("session", args, map[string]func([]string) error{
			"new": cliSessionNew, "ls": cliSessionLs, "list": cliSessionLs, "inspect": cliSessionInspect,
			"rename": cliSessionRename, "move": cliSessionMove, "kill": cliSessionKill,
		})
	case "window":
		return sub("window", args, map[string]func([]string) error{
			"new": cliWindowNew, "ls": cliWindowLs, "list": cliWindowLs, "inspect": cliWindowInspect,
			"focus": cliWindowFocus, "move": cliWindowMove, "rename": cliWindowRename, "close": cliWindowClose,
		})
	case "block":
		return sub("block", args, map[string]func([]string) error{
			"ls": cliBlockLs, "list": cliBlockLs, "inspect": cliBlockInspect, "focus": cliBlockFocus,
			"send": cliSend, "send-key": cliSendKey, "capture": cliCapture, "zoom": cliZoom, "move": cliMove,
			"swap": cliSwap, "rename": cliBlockRename, "close": cliBlockClose, "types": cliBlockTypes,
			"new": cliBlockNew,
		})
	case "client":
		return sub("client", args, map[string]func([]string) error{"ls": cliClientLs, "list": cliClientLs, "inspect": cliClientInspect})
	case "status":
		return cliStatus(args)
	case "events":
		return cliEvents(args)
	case "wait":
		return cliWait(args)
	case "do":
		return cliDo(args)
	case "actions":
		return cliActions(args)
	case "keymap":
		return cliKeymap(args)
	case "config":
		return sub("config", args, map[string]func([]string) error{"check": cliConfigCheck, "reload": cliConfigReload})
	case "skill":
		return sub("skill", args, map[string]func([]string) error{
			"setup": cliSkillSetup, "status": cliSkillStatus, "remove": cliSkillRemove, "show": cliSkillShow,
			"hook": cliSkillHook,
		})
	case "server":
		return sub("server", args, map[string]func([]string) error{
			"status": cliServerStatus, "stop": cliServerStop, "logs": cliServerLogs, "start": cliServerStart,
			"run": cliServerRun,
		})
	case "completion":
		return cliCompletion(args)
	}
	return usagef("unknown command %q: run \"rex help\"", cmd)
}

func sub(group string, args []string, cmds map[string]func([]string) error) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(group)
		return nil
	}
	if f, ok := cmds[args[0]]; ok {
		return f(args[1:])
	}
	return usagef("unknown command \"%s %s\": run \"rex help %s\"", group, args[0], group)
}

// Flags.

// flags parses a command's flags, which may come before, between and
// after its arguments, as "--name value", "--name=value" or "-n value";
// what follows "--" is a command to run.
type flags struct {
	strs    map[string]*string
	bools   map[string]*bool
	names   map[string]string // every spelling to the flag's name
	args    []string
	command []string
	dashed  bool
}

func newFlags() *flags {
	return &flags{strs: map[string]*string{}, bools: map[string]*bool{}, names: map[string]string{}}
}

// str declares a flag of a value, by its spellings: "s,session".
func (f *flags) str(spellings string, p *string) *flags {
	names := strings.Split(spellings, ",")
	for _, n := range names {
		f.names[n] = names[len(names)-1]
	}
	f.strs[names[len(names)-1]] = p
	return f
}

func (f *flags) boolean(spellings string, p *bool) *flags {
	names := strings.Split(spellings, ",")
	for _, n := range names {
		f.names[n] = names[len(names)-1]
	}
	f.bools[names[len(names)-1]] = p
	return f
}

// targets declares the flags of the targets: -s, -w, -b.
func (f *flags) targets(a *rex.Args) *flags {
	return f.str("s,session", &a.Session).str("w,window", &a.Window).str("b,block", &a.Block)
}

func (f *flags) parse(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			f.command, f.dashed = args[i+1:], true
			return nil
		}
		if len(arg) < 2 || arg[0] != '-' || isNumber(arg) {
			f.args = append(f.args, arg)
			continue
		}
		name, val, hasVal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		key, ok := f.names[name]
		if !ok {
			return usagef("unknown flag %s", arg)
		}
		if p, ok := f.bools[key]; ok {
			switch {
			case !hasVal:
				*p = true
			case val == "true" || val == "1":
				*p = true
			case val == "false" || val == "0":
				*p = false
			default:
				return usagef("%s takes true or false", arg)
			}
			continue
		}
		if !hasVal {
			if i+1 >= len(args) {
				return usagef("%s needs a value", arg)
			}
			i++
			val = args[i]
		}
		*f.strs[key] = val
	}
	return nil
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// The connection.

// connect connects to the server: it starts one when none runs, unless
// --autostart=false.
func connect() (*rex.Client, error) {
	var c *rex.Client
	var err error
	if opts.autostart {
		c, err = rex.Connect()
	} else {
		var nc net.Conn
		if nc, err = net.DialTimeout("unix", rex.SocketPath(), opts.timeout); err == nil {
			nc.Close()
			c, err = rex.Connect()
		}
	}
	if err != nil {
		return nil, &cliError{exitNoServer, fmt.Errorf("could not reach the server at %s: %w", rex.SocketPath(), err)}
	}
	h, err := c.Hello()
	if err != nil {
		c.Close()
		return nil, &cliError{exitNoServer, err}
	}
	if h.Version != rex.ProtocolVersion {
		c.Close()
		return nil, &cliError{exitFailed, fmt.Errorf("the server runs another version of GoRex (protocol %d, this rex speaks %d): open GoRex to restart it",
			h.Version, rex.ProtocolVersion)}
	}
	return c, nil
}

// withContext fills in where the command runs, from the environment.
func withContext(a *rex.Args) *rex.Args {
	a.CtxSession, a.CtxBlock = os.Getenv("REX_SESSION"), os.Getenv("REX_BLOCK")
	if a.CtxBlock == "" {
		a.CtxBlock = os.Getenv("GOREX_SESSION")
	}
	return a
}

// do sends a request with the command's context.
func do(op string, a *rex.Args, out any) error {
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	return c.DoFor(op, withContext(a), out, opts.timeout)
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// raw prints an answer as JSON, or does what text says with it.
func output(jsonOut bool, v any, text func()) {
	if jsonOut {
		printJSON(v)
		return
	}
	text()
}

func table() *tabwriter.Writer { return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) }

func ago(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return roundDur(time.Since(t)) + " ago"
}

func short(id string, full bool) string {
	if full {
		return id
	}
	return rex.ShortID(id)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// Commands.

func cliVersion(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	v := map[string]any{"version": rex.Version, "protocol": rex.ProtocolVersion, "name": "GoRex"}
	if c, err := rex.Dial(); err == nil {
		if h, err := c.Hello(); err == nil {
			v["server"] = map[string]any{"pid": h.PID, "protocol": h.Version, "started": h.Started, "exe": h.Exe}
		}
		c.Close()
	}
	output(jsonOut, v, func() {
		fmt.Printf("rex %s (GoRex, protocol %d)\n", rex.Version, rex.ProtocolVersion)
		if s, ok := v["server"].(map[string]any); ok {
			fmt.Printf("server pid %v, protocol %v\n", s["pid"], s["protocol"])
		}
	})
	return nil
}

// createFlags are the flags of commands that start a terminal.
func createFlags(f *flags, o *rex.CreateOptions) *flags {
	return f.str("c,cwd,d,dir", &o.Dir).boolean("keep-open", &o.KeepOpen).str("shell", &o.Shell).
		boolean("interactive", &o.Interactive)
}

func absPath(p string) (string, error) { return filepath.Abs(p) }

func fixDir(o *rex.CreateOptions) {
	if o.Dir != "" {
		if strings.HasPrefix(o.Dir, "~") {
			home, _ := os.UserHomeDir()
			o.Dir = home + o.Dir[1:]
		}
		if abs, err := absPath(o.Dir); err == nil {
			o.Dir = abs
		}
	}
}

func cliSessionNew(args []string) error {
	var o rex.CreateOptions
	var a rex.Args
	var jsonOut bool
	f := createFlags(newFlags(), &o).str("window", &a.WindowLabel).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 1 {
		return usagef("usage: rex new [LABEL] [-- COMMAND...]")
	}
	if len(f.args) == 1 {
		a.Label = f.args[0]
	}
	o.Command = f.command
	if o.Dir == "" {
		o.Dir, _ = os.Getwd()
	}
	fixDir(&o)
	a.Create = &o
	var res map[string]any
	if err := do("session.new", &a, &res); err != nil {
		return err
	}
	output(jsonOut, res, func() { fmt.Println(res["session_id"]) })
	return nil
}

func cliSessionLs(args []string) error {
	var jsonOut, quiet bool
	shortIDs := true
	if err := newFlags().boolean("json", &jsonOut).boolean("q,quiet", &quiet).boolean("short", &shortIDs).parse(args); err != nil {
		return err
	}
	m, err := getModel()
	if err != nil {
		return err
	}
	if jsonOut {
		var out []map[string]any
		for _, ss := range m.Sessions {
			out = append(out, sessionJSON(m, ss))
		}
		printJSON(map[string]any{"sessions": out})
		return nil
	}
	if quiet {
		for _, ss := range m.Sessions {
			fmt.Println(short(ss.ID, !shortIDs))
		}
		return nil
	}
	here := os.Getenv("REX_SESSION")
	w := table()
	fmt.Fprintln(w, "  ID\tLABEL\tWINDOWS\tBLOCKS\tCREATED")
	for _, ss := range m.Sessions {
		mark := " "
		if ss.ID == here {
			mark = "*"
		}
		n := 0
		for _, win := range ss.Windows {
			n += len(win.Layout.Blocks())
		}
		fmt.Fprintf(w, "%s %s\t%s\t%d\t%d\t%s\n", mark, short(ss.ID, !shortIDs), dash(ss.Label), len(ss.Windows), n, ago(ss.Created))
	}
	return w.Flush()
}

func sessionJSON(m *rex.Model, ss *rex.Session) map[string]any {
	var ws []map[string]any
	for i, w := range ss.Windows {
		ws = append(ws, windowJSON(w, i))
	}
	return map[string]any{"session_id": ss.ID, "label": ss.Label, "created": ss.Created,
		"active_window_id": ss.Active, "windows": ws}
}

func windowJSON(w *rex.Window, i int) map[string]any {
	return map[string]any{"window_id": w.ID, "label": w.Label, "index": i + 1, "block_ids": w.Layout.Blocks(),
		"focused_block_id": w.Focus, "zoomed_block_id": w.Zoom, "layout": w.Layout}
}

func getModel() (*rex.Model, error) {
	c, err := connect()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return c.Model()
}

// resolveSessionID resolves a session target with the server's rules.
func resolveIDs(op string, a *rex.Args) (map[string]any, error) {
	var res map[string]any
	err := do(op, a, &res)
	return res, err
}

func cliSessionInspect(args []string) error {
	var a rex.Args
	var jsonOut bool
	f := newFlags().boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Session = f.args[0]
	}
	m, ss, err := findSession(&a)
	if err != nil {
		return err
	}
	j := sessionJSON(m, ss)
	output(jsonOut, j, func() {
		fmt.Printf("session  %s\nlabel    %s\ncreated  %s\nwindows  %d\n", ss.ID, dash(ss.Label), ss.Created.Format(time.RFC3339), len(ss.Windows))
		for i, w := range ss.Windows {
			mark := " "
			if w.ID == ss.Active {
				mark = "*"
			}
			fmt.Printf("  %s %d %s  %s  %d blocks\n", mark, i+1, rex.ShortID(w.ID), dash(w.Label), len(w.Layout.Blocks()))
		}
	})
	return nil
}

// findSession finds a session by the server's rules: it asks the server
// to resolve the target, then finds it in the model.
func findSession(a *rex.Args) (*rex.Model, *rex.Session, error) {
	c, err := connect()
	if err != nil {
		return nil, nil, err
	}
	defer c.Close()
	var res map[string]any
	if err := c.Do("session.resolve", withContext(a), &res); err != nil {
		return nil, nil, err
	}
	m, err := c.Model()
	if err != nil {
		return nil, nil, err
	}
	ss := m.Session(res["session_id"].(string))
	if ss == nil {
		return nil, nil, fmt.Errorf("the session went away")
	}
	return m, ss, nil
}

func cliSessionRename(args []string) error {
	var a rex.Args
	f := newFlags()
	if err := f.parse(args); err != nil {
		return err
	}
	switch len(f.args) {
	case 1:
		a.Label = f.args[0]
	case 2:
		a.Session, a.Label = f.args[0], f.args[1]
	default:
		return usagef("usage: rex session rename [SESSION] LABEL")
	}
	_, err := resolveIDs("session.rename", &a)
	return err
}

func cliSessionMove(args []string) error {
	var a rex.Args
	var index string
	f := newFlags().str("before", &a.Before).str("after", &a.After).str("index", &index)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Session = f.args[0]
	}
	if n := countSet(a.Before != "", a.After != "", index != ""); n != 1 {
		return usagef("exactly one of --before, --after or --index is needed")
	}
	if index != "" {
		n, err := strconv.Atoi(index)
		if err != nil || n < 1 {
			return usagef("--index takes a position from 1")
		}
		a.Index = n
	}
	_, err := resolveIDs("session.move", &a)
	return err
}

func countSet(bs ...bool) int {
	n := 0
	for _, b := range bs {
		if b {
			n++
		}
	}
	return n
}

func cliSessionKill(args []string) error {
	f := newFlags()
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) == 0 {
		return usagef("usage: rex kill SESSION...")
	}
	for _, s := range f.args {
		if _, err := resolveIDs("session.kill", &rex.Args{Session: s}); err != nil {
			return err
		}
	}
	return nil
}

func cliWindowNew(args []string) error {
	var o rex.CreateOptions
	var a rex.Args
	var jsonOut bool
	focus := true
	f := createFlags(newFlags(), &o).str("s,session", &a.Session).boolean("focus", &focus).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 1 {
		// GoRex's first new-tab took the command without "--".
		f.command = append(f.args, f.command...)
		f.args = nil
	}
	if len(f.args) == 1 {
		a.WindowLabel = f.args[0]
	}
	o.Command = f.command
	if o.Dir == "" && len(o.Command) > 0 {
		o.Dir, _ = os.Getwd()
	}
	fixDir(&o)
	a.Create, a.NoFocus = &o, !focus
	var res map[string]any
	if err := do("window.new", &a, &res); err != nil {
		return err
	}
	output(jsonOut, res, func() { fmt.Println(res["window_id"]) })
	return nil
}

func cliWindowLs(args []string) error {
	var a rex.Args
	var jsonOut, quiet, all bool
	shortIDs := true
	f := newFlags().str("s,session", &a.Session).boolean("A,all", &all).boolean("json", &jsonOut).
		boolean("q,quiet", &quiet).boolean("short", &shortIDs)
	if err := f.parse(args); err != nil {
		return err
	}
	var sessions []*rex.Session
	var m *rex.Model
	if all {
		var err error
		if m, err = getModel(); err != nil {
			return err
		}
		sessions = m.Sessions
	} else {
		var ss *rex.Session
		var err error
		if m, ss, err = findSession(&a); err != nil {
			return err
		}
		sessions = []*rex.Session{ss}
	}
	if jsonOut {
		var out []map[string]any
		for _, ss := range sessions {
			for i, w := range ss.Windows {
				j := windowJSON(w, i)
				j["session_id"] = ss.ID
				out = append(out, j)
			}
		}
		printJSON(map[string]any{"windows": out})
		return nil
	}
	w := table()
	if !quiet {
		fmt.Fprintln(w, "  #\tID\tLABEL\tSESSION\tBLOCKS\tFOCUSED")
	}
	for _, ss := range sessions {
		for i, win := range ss.Windows {
			if quiet {
				fmt.Fprintln(w, short(win.ID, !shortIDs))
				continue
			}
			mark := " "
			if win.ID == ss.Active {
				mark = "*"
			}
			fmt.Fprintf(w, "%s %d\t%s\t%s\t%s\t%d\t%s\n", mark, i+1, short(win.ID, !shortIDs), dash(win.Label),
				m.SessionName(ss), len(win.Layout.Blocks()), short(win.Focus, !shortIDs))
		}
	}
	return w.Flush()
}

// findWindow resolves a window by the server's rules.
func findWindow(a *rex.Args) (*rex.Model, *rex.Session, *rex.Window, error) {
	c, err := connect()
	if err != nil {
		return nil, nil, nil, err
	}
	defer c.Close()
	var res map[string]any
	if err := c.Do("window.resolve", withContext(a), &res); err != nil {
		return nil, nil, nil, err
	}
	m, err := c.Model()
	if err != nil {
		return nil, nil, nil, err
	}
	ss := m.Session(res["session_id"].(string))
	for _, w := range ss.Windows {
		if w.ID == res["window_id"] {
			return m, ss, w, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("the window went away")
}

func cliWindowInspect(args []string) error {
	var a rex.Args
	var jsonOut bool
	f := newFlags().str("s,session", &a.Session).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Window = f.args[0]
	}
	m, ss, w, err := findWindow(&a)
	if err != nil {
		return err
	}
	i := slices.Index(ss.Windows, w)
	j := windowJSON(w, i)
	j["session_id"] = ss.ID
	output(jsonOut, j, func() {
		fmt.Printf("window   %s\nlabel    %s\nsession  %s (%s)\nindex    %d\nfocused  %s\n", w.ID, dash(w.Label), ss.ID, m.SessionName(ss), i+1, dash(w.Focus))
		if w.Zoom != "" {
			fmt.Printf("zoomed   %s\n", w.Zoom)
		}
		fmt.Println("blocks")
		for _, id := range w.Layout.Blocks() {
			b := m.Blocks[id]
			label := ""
			if b != nil {
				label = b.Label
			}
			fmt.Printf("  %s  %s\n", id, label)
		}
	})
	return nil
}

func cliWindowFocus(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session).boolean("wrap", &a.Wrap)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Target = f.args[0]
	}
	_, err := resolveIDs("window.focus", &a)
	return err
}

func cliWindowMove(args []string) error {
	var a rex.Args
	var index string
	f := newFlags().str("s,session", &a.Session).str("before", &a.Before).str("after", &a.After).str("index", &index)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Window = f.args[0]
	}
	if n := countSet(a.Before != "", a.After != "", index != ""); n != 1 {
		return usagef("exactly one of --before, --after or --index is needed")
	}
	if index != "" {
		n, err := strconv.Atoi(index)
		if err != nil || n < 1 {
			return usagef("--index takes a position from 1")
		}
		a.Index = n
	}
	_, err := resolveIDs("window.move", &a)
	return err
}

func cliWindowRename(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session)
	if err := f.parse(args); err != nil {
		return err
	}
	switch len(f.args) {
	case 1:
		a.Label = f.args[0]
	case 2:
		a.Window, a.Label = f.args[0], f.args[1]
	default:
		return usagef("usage: rex window rename [WINDOW] LABEL")
	}
	_, err := resolveIDs("window.rename", &a)
	return err
}

func cliWindowClose(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) == 0 {
		return usagef("usage: rex window close WINDOW...")
	}
	for _, w := range f.args {
		a2 := a
		a2.Window = w
		if _, err := resolveIDs("window.close", &a2); err != nil {
			return err
		}
	}
	return nil
}

// cliRun is "rex run" and "rex split": a terminal in a new window, or
// beside a block.
func cliRun(args []string, split bool) error {
	var o rex.CreateOptions
	var a rex.Args
	var dir, ratio string
	var jsonOut, wait, down, right bool
	focus := true
	f := createFlags(newFlags(), &o).targets(&a).str("split", &dir).str("ratio", &ratio).str("label", &o.Label).
		boolean("focus", &focus).boolean("json", &jsonOut).boolean("wait", &wait).
		// GoRex's first split's flags.
		str("t,target", &a.Block).boolean("v,vertical,down,below", &down).boolean("horizontal,right,beside", &right)
	if err := f.parse(args); err != nil {
		return err
	}
	cmd := f.command
	if !f.dashed && len(f.args) > 0 {
		cmd = f.args // GoRex's first split took the command without "--"
	} else if len(f.args) > 0 {
		return usagef("unexpected %q: the command goes after --", f.args[0])
	}
	if !split && len(cmd) == 0 {
		return usagef("usage: rex run [flags] -- COMMAND...")
	}
	if down {
		dir = "below"
	} else if right && dir == "" {
		dir = "right"
	}
	o.Command = cmd
	if o.Dir == "" && len(cmd) > 0 && !split {
		o.Dir, _ = os.Getwd()
	}
	fixDir(&o)
	if ratio != "" {
		r, err := strconv.ParseFloat(strings.TrimSuffix(ratio, "%"), 64)
		if err != nil || r <= 0 || r >= 100 {
			return usagef("--ratio takes a percentage between 1 and 99")
		}
		a.Ratio = float32(r / 100)
	}
	a.Create, a.NoFocus, a.Direction = &o, !focus, dir
	op := "block.split"
	if !split && dir == "" {
		op = "window.new"
		a.Block = ""
	} else if split && dir == "" {
		a.Direction = "right"
	}
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	var res map[string]any
	if err := c.DoFor(op, withContext(&a), &res, opts.timeout); err != nil {
		return err
	}
	if jsonOut {
		printJSON(res)
	} else {
		fmt.Println(res["block_id"])
	}
	if !wait {
		return nil
	}
	var w map[string]any
	if err := c.DoFor("wait", &rex.Args{Block: res["block_id"].(string)}, &w, 0); err != nil {
		return err
	}
	if code, _ := w["exit_code"].(float64); code != 0 {
		return exitWith(int(code))
	}
	return nil
}

func cliBlockNew(args []string) error {
	return usagef("GoRex's blocks are terminals: rex run or rex split start one")
}

func cliBlockTypes(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	types := []map[string]string{{"flavor": "com.superlogical.terminal.shell", "title": "Terminal"}}
	output(jsonOut, map[string]any{"types": types}, func() { fmt.Println("com.superlogical.terminal.shell  Terminal") })
	return nil
}

func cliSend(args []string) error {
	var a rex.Args
	f := newFlags().targets(&a)
	if err := f.parse(args); err != nil {
		return err
	}
	text := strings.Join(append(f.args, f.command...), " ")
	if len(f.args) == 0 && len(f.command) == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = string(b)
	}
	a.Data = text
	_, err := resolveIDs("send", &a)
	return err
}

func cliSendKey(args []string) error {
	var a rex.Args
	var action string
	f := newFlags().targets(&a).str("action", &action)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) == 0 {
		return usagef("usage: rex send-key KEY...")
	}
	if action == "release" {
		return nil // a terminal sends nothing as a key is released
	}
	a.Keys = f.args
	_, err := resolveIDs("sendKey", &a)
	return err
}

// cliSendKeysCompat is GoRex's first send-keys: text with <CR> and <Esc>.
func cliSendKeysCompat(args []string) error {
	var a rex.Args
	switch len(args) {
	case 0:
		return usagef("usage: rex send-keys [BLOCK] TEXT")
	case 1:
		a.Data = args[0]
	default:
		a.Block, a.Data = args[0], strings.Join(args[1:], " ")
	}
	r := strings.NewReplacer("<CR>", "\r", "<Esc>", "\x1b", `\n`, "\n", `\r`, "\r", `\e`, "\x1b")
	a.Data = r.Replace(a.Data)
	_, err := resolveIDs("send", &a)
	return err
}

func cliCapture(args []string) error {
	var a rex.Args
	var unwrap bool
	f := newFlags().targets(&a).str("format", &a.Format).boolean("trim", &a.Trim).boolean("unwrap", &unwrap)
	if err := f.parse(args); err != nil {
		return err
	}
	if a.Format == "html" {
		return usagef("GoRex captures text or vt, not html")
	}
	var res map[string]any
	if err := do("capture", &a, &res); err != nil {
		return err
	}
	fmt.Print(res["text"])
	return nil
}

func cliFocus(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window).boolean("wrap", &a.Wrap)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) != 1 {
		return usagef("usage: rex focus TARGET|DIRECTION")
	}
	a.Target = f.args[0]
	_, err := resolveIDs("focus", &a)
	return err
}

func cliBlockFocus(args []string) error {
	var a rex.Args
	f := newFlags().targets(&a)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Block = f.args[0]
	}
	_, err := resolveIDs("block.focus", &a)
	return err
}

func cliZoom(args []string) error {
	var a rex.Args
	var on, off bool
	f := newFlags().targets(&a).boolean("on", &on).boolean("off", &off)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Block = f.args[0]
	}
	switch {
	case on && off:
		return usagef("--on or --off, not both")
	case on:
		a.Zoom = "on"
	case off:
		a.Zoom = "off"
	}
	_, err := resolveIDs("block.zoom", &a)
	return err
}

func cliMove(args []string) error {
	var a rex.Args
	var ratio string
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window).str("ratio", &ratio)
	if err := f.parse(args); err != nil {
		return err
	}
	switch len(f.args) {
	case 2:
		a.Direction, a.Target = f.args[0], f.args[1]
	case 3:
		a.Block, a.Direction, a.Target = f.args[0], f.args[1], f.args[2]
	default:
		return usagef("usage: rex move [BLOCK] DIRECTION ANCHOR")
	}
	if ratio != "" {
		r, err := strconv.ParseFloat(strings.TrimSuffix(ratio, "%"), 64)
		if err != nil || r < 1 || r > 99 {
			return usagef("--ratio takes a percentage between 1 and 99")
		}
		a.Ratio = float32(r / 100)
	}
	_, err := resolveIDs("block.move", &a)
	return err
}

func cliSwap(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) != 2 {
		return usagef("usage: rex swap BLOCK OTHER")
	}
	a.Block, a.Target = f.args[0], f.args[1]
	_, err := resolveIDs("block.swap", &a)
	return err
}

func cliResize(args []string) error {
	var a rex.Args
	var amount string
	f := newFlags().targets(&a).str("amount", &amount).str("direction", &a.Direction)
	if err := f.parse(args); err != nil {
		return err
	}
	switch len(f.args) {
	case 1:
		a.Direction = f.args[0]
	case 2:
		a.Block, a.Direction = f.args[0], f.args[1]
	}
	if a.Direction == "" {
		return usagef("usage: rex resize [BLOCK] DIRECTION [--amount PCT]")
	}
	if amount != "" {
		n, err := strconv.ParseFloat(strings.TrimSuffix(amount, "%"), 64)
		if err != nil || n <= 0 {
			return usagef("--amount takes a percentage")
		}
		a.Amount = float32(n)
	}
	_, err := resolveIDs("block.resize", &a)
	return err
}

func cliBlockRename(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window)
	if err := f.parse(args); err != nil {
		return err
	}
	switch len(f.args) {
	case 1:
		a.Label = f.args[0]
	case 2:
		a.Block, a.Label = f.args[0], f.args[1]
	default:
		return usagef("usage: rex block rename [BLOCK] LABEL")
	}
	_, err := resolveIDs("block.rename", &a)
	return err
}

func cliBlockClose(args []string) error {
	var a rex.Args
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) == 0 {
		// GoRex's first close closed the terminal it ran in.
		_, err := resolveIDs("block.close", &a)
		return err
	}
	for _, b := range f.args {
		a2 := a
		a2.Block = b
		if _, err := resolveIDs("block.close", &a2); err != nil {
			return err
		}
	}
	return nil
}

// blockRow is a block, as listings show it.
type blockRow struct {
	rex.BlockInfo
	SessionName string `json:"session_label,omitempty"`
	Index       int    `json:"window_index,omitempty"`
	Focused     bool   `json:"focused"`
}

func blockRows() (*rex.Model, []blockRow, error) {
	c, err := connect()
	if err != nil {
		return nil, nil, err
	}
	defer c.Close()
	m, err := c.Model()
	if err != nil {
		return nil, nil, err
	}
	infos, err := c.List()
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]rex.BlockInfo{}
	for _, in := range infos {
		byID[in.ID] = in
	}
	var rows []blockRow
	for _, ss := range m.Sessions {
		for i, w := range ss.Windows {
			for _, id := range w.Layout.Blocks() {
				rows = append(rows, blockRow{BlockInfo: byID[id], SessionName: m.SessionName(ss), Index: i + 1, Focused: w.Focus == id})
			}
		}
	}
	return m, rows, nil
}

func cliBlockLs(args []string) error {
	var a rex.Args
	var jsonOut, quiet, all bool
	shortIDs := true
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window).boolean("A,all", &all).
		boolean("json", &jsonOut).boolean("q,quiet", &quiet).boolean("short", &shortIDs)
	if err := f.parse(args); err != nil {
		return err
	}
	_, rows, err := blockRows()
	if err != nil {
		return err
	}
	if !all {
		// The session, or window, the command acts in.
		var keep func(r blockRow) bool
		if a.Window != "" {
			_, _, w, err := findWindow(&a)
			if err != nil {
				return err
			}
			keep = func(r blockRow) bool { return r.Window == w.ID }
		} else {
			_, ss, err := findSession(&a)
			if err != nil {
				return err
			}
			keep = func(r blockRow) bool { return r.Session == ss.ID }
		}
		rows = slices.DeleteFunc(rows, func(r blockRow) bool { return !keep(r) })
	}
	if jsonOut {
		printJSON(map[string]any{"blocks": rows})
		return nil
	}
	here := os.Getenv("REX_BLOCK")
	w := table()
	if !quiet {
		fmt.Fprintln(w, "  ID\tLABEL\tWINDOW\tPROGRAM\tSTATUS\tPID\tSIZE\tDIR")
	}
	for _, r := range rows {
		if quiet {
			fmt.Fprintln(w, short(r.ID, !shortIDs))
			continue
		}
		mark := " "
		switch {
		case r.ID == here:
			mark = "@"
		case r.Focused:
			mark = "*"
		}
		prog := r.Program
		if r.Exited {
			prog = fmt.Sprintf("exited (%d)", r.ExitCode)
		}
		st := "-"
		if sum := rex.StatusSummary(r.Status); sum != nil {
			st = sum.State
			if sum.Kind != "" {
				st += "(" + sum.Kind + ")"
			}
		}
		fmt.Fprintf(w, "%s %s\t%s\t%d\t%s\t%s\t%d\t%dx%d\t%s\n", mark, short(r.ID, !shortIDs), dash(r.Label), r.Index, prog, st, r.PID, r.Cols, r.Rows, shortDir(r.Dir))
	}
	return w.Flush()
}

func cliBlockInspect(args []string) error {
	var a rex.Args
	var jsonOut bool
	f := newFlags().targets(&a).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) > 0 {
		a.Block = f.args[0]
	}
	var res map[string]any
	if err := do("block.resolve", &a, &res); err != nil {
		return err
	}
	_, rows, err := blockRows()
	if err != nil {
		return err
	}
	i := slices.IndexFunc(rows, func(r blockRow) bool { return r.ID == res["block_id"] })
	if i < 0 {
		return fmt.Errorf("the block went away")
	}
	r := rows[i]
	output(jsonOut, r, func() {
		w := table()
		fmt.Fprintf(w, "block\t%s\n", r.ID)
		fmt.Fprintf(w, "label\t%s\n", dash(r.Label))
		fmt.Fprintf(w, "session\t%s (%s)\n", r.Session, r.SessionName)
		fmt.Fprintf(w, "window\t%s (%d)\n", r.Window, r.Index)
		fmt.Fprintf(w, "focused\t%v\n", r.Focused)
		fmt.Fprintf(w, "pid\t%d\n", r.PID)
		fmt.Fprintf(w, "shell\t%s\n", r.Shell)
		fmt.Fprintf(w, "program\t%s %s\n", r.Program, strings.Join(r.Args[min(1, len(r.Args)):], " "))
		fmt.Fprintf(w, "idle\t%v\n", r.Idle)
		fmt.Fprintf(w, "dir\t%s\n", r.Dir)
		fmt.Fprintf(w, "size\t%dx%d\n", r.Cols, r.Rows)
		if r.Title != "" {
			fmt.Fprintf(w, "title\t%s\n", r.Title)
		}
		fmt.Fprintf(w, "output\t%d bytes, last %s\n", r.Output, ago(r.LastOutput))
		fmt.Fprintf(w, "attached\t%d\n", r.Attached)
		if r.Exited {
			fmt.Fprintf(w, "exited\tstatus %d\n", r.ExitCode)
		}
		for _, st := range r.Status {
			fmt.Fprintf(w, "status\t%s\n", statusLine(st))
		}
		w.Flush()
	})
	return nil
}

// cliTree is GoRex's first list: the sessions, their windows and blocks.
func cliTree(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	m, rows, err := blockRows()
	if err != nil {
		return err
	}
	if jsonOut {
		printJSON(map[string]any{"model": m, "blocks": rows})
		return nil
	}
	byID := map[string]blockRow{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	for _, ss := range m.Sessions {
		fmt.Printf("%s  %s\n", rex.ShortID(ss.ID), m.SessionName(ss))
		for i, w := range ss.Windows {
			mark := " "
			if w.ID == ss.Active {
				mark = "*"
			}
			fmt.Printf("  %s [%d] %s  %s\n", mark, i+1, rex.ShortID(w.ID), dash(w.Label))
			for _, id := range w.Layout.Blocks() {
				r := byID[id]
				pm := "  "
				if r.Focused {
					pm = " >"
				}
				fmt.Printf("    %s %s  pid:%-6d  %-12s  %dx%d  %s\n", pm, id, r.PID, r.Program, r.Cols, r.Rows, shortDir(r.Dir))
			}
		}
	}
	return nil
}

func cliClientLs(args []string) error {
	var jsonOut, all bool
	if err := newFlags().boolean("json", &jsonOut).boolean("all", &all).parse(args); err != nil {
		return err
	}
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	cs, err := c.Clients()
	if err != nil {
		return err
	}
	if jsonOut {
		printJSON(map[string]any{"clients": cs})
		return nil
	}
	w := table()
	fmt.Fprintln(w, "ID\tKIND\tNAME\tAGE\tVIEWING")
	for _, cl := range cs {
		if cl.Kind == "cli" && !all {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", rex.ShortID(cl.ID), cl.Kind, dash(cl.Name), ago(cl.Connected), short(cl.Viewing, false))
	}
	return w.Flush()
}

func cliClientInspect(args []string) error {
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	cs, err := c.Clients()
	if err != nil {
		return err
	}
	for _, cl := range cs {
		if len(args) == 0 && cl.Kind == "app" || len(args) > 0 && (cl.ID == args[0] || strings.HasSuffix(cl.ID, args[0]) || cl.Name == args[0]) {
			printJSON(cl)
			return nil
		}
	}
	return &cliError{exitTarget, errors.New("client not found")}
}

func cliEvents(args []string) error {
	var a rex.Args
	var jsonOut bool
	f := newFlags().str("s,session", &a.Session).boolean("all", &a.All).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	out := bufio.NewWriter(os.Stdout)
	events := make(chan rex.Event, 256)
	c.OnEvent = func(e rex.Event) { events <- e }
	if err := c.Subscribe(a); err != nil {
		return err
	}
	for {
		select {
		case <-c.Closed():
			out.Flush()
			return &cliError{exitNoServer, errors.New("the server went away")}
		case e := <-events:
			if jsonOut {
				b, _ := json.Marshal(e)
				out.Write(append(b, '\n'))
			} else {
				out.WriteString(eventLine(e) + "\n")
			}
			out.Flush()
		}
	}
}

// eventLine writes an event as a line for people.
func eventLine(e rex.Event) string {
	typ, subject, rest := e.Type, "", ""
	p := e.Payload
	if e.Type == "session_event" {
		inner, _ := p["event"].(map[string]any)
		typ, _ = inner["type"].(string)
		p, _ = inner["payload"].(map[string]any)
		rest = "session " + rex.ShortID(e.SessionID)
	}
	if typ == "block_event" {
		name, _ := p["name"].(string)
		typ = "terminal." + name
		if bp, ok := p["payload"].(map[string]any); ok && len(bp) > 0 {
			b, _ := json.Marshal(bp)
			rest = string(b) + "  " + rest
		}
	}
	for _, k := range []string{"block_id", "window_id", "session_id", "client_id"} {
		if v, ok := p[k].(string); ok && subject == "" {
			subject = rex.ShortID(v)
		}
	}
	if s, ok := p["session"].(map[string]any); ok && subject == "" {
		subject = rex.ShortID(fmt.Sprint(s["session_id"]))
		if l, _ := s["label"].(string); l != "" {
			rest = l
		}
	}
	for _, k := range []string{"label", "revision"} {
		if v, ok := p[k]; ok && typ != "block_event" {
			rest = fmt.Sprintf("%s %v  %s", k, v, rest)
		}
	}
	return fmt.Sprintf("%s  %-22s %-8s %s", e.Time.Local().Format("15:04:05"), typ, subject, strings.TrimSpace(rest))
}

func cliWait(args []string) error {
	var a rex.Args
	var forDur string
	var jsonOut bool
	f := newFlags().str("s,session", &a.Session).str("w,window", &a.Window).str("for", &forDur).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) != 1 {
		return usagef("usage: rex wait TARGET")
	}
	a.Target = f.args[0]
	var d time.Duration
	if forDur != "" {
		var err error
		if d, err = time.ParseDuration(forDur); err != nil {
			return usagef("--for: %v", err)
		}
	}
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	var res map[string]any
	if err := c.DoFor("wait", withContext(&a), &res, d); err != nil {
		if strings.Contains(err.Error(), "timed out") {
			return &cliError{exitFailed, fmt.Errorf("gave up after %s", d)}
		}
		return err
	}
	if jsonOut {
		b, _ := json.Marshal(res)
		fmt.Println(string(b))
	}
	if code, _ := res["exit_code"].(float64); code != 0 {
		return exitWith(int(code))
	}
	return nil
}

func cliDo(args []string) error {
	var a rex.Args
	var argsJSON, eval, action string
	var jsonOut bool
	f := newFlags().str("s,session", &a.Session).str("b,block", &a.Block).str("args", &argsJSON).
		str("e,eval", &eval).str("a,action", &action).boolean("json", &jsonOut)
	var ignored string
	f.str("origin", &ignored).str("for", &ignored)
	var all, failFast bool
	f.boolean("all", &all).boolean("fail-fast", &failFast)
	if err := f.parse(args); err != nil {
		return err
	}
	if eval != "" || len(f.args) > 0 && (strings.HasSuffix(f.args[0], ".lua") || strings.ContainsAny(f.args[0], `/\`) || f.args[0] == "-") {
		return usagef("GoRex runs no Lua: rex do performs the app's actions (rex actions lists them)")
	}
	if len(f.args) == 0 {
		return usagef("usage: rex do ACTION [KEY=VALUE...]")
	}
	a.Action = f.args[0]
	params := map[string]any{}
	if argsJSON != "" {
		var b []byte
		var err error
		switch {
		case argsJSON == "-":
			b, err = io.ReadAll(os.Stdin)
		case strings.HasPrefix(argsJSON, "@"):
			b, err = os.ReadFile(argsJSON[1:])
		default:
			b = []byte(argsJSON)
		}
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &params); err != nil {
			return usagef("--args: %v", err)
		}
	}
	for _, kv := range f.args[1:] {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return usagef("arguments are KEY=VALUE, not %q", kv)
		}
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			params[k] = n
		} else if v == "true" || v == "false" {
			params[k] = v == "true"
		} else {
			params[k] = v
		}
	}
	a.Params, _ = json.Marshal(params)
	a.Client = opts.client
	var res json.RawMessage
	if err := do("client.do", &a, &res); err != nil {
		return err
	}
	if jsonOut && len(res) > 0 {
		var v any
		_ = json.Unmarshal(res, &v) // the server's JSON; null if it were not
		printJSON(v)
	}
	return nil
}

func cliActions(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	list := actionList()
	output(jsonOut, map[string]any{"actions": list}, func() {
		w := table()
		fmt.Fprintln(w, "NAME\tTITLE\tARGS")
		for _, x := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", x.Name, x.Title, x.Args)
		}
		w.Flush()
	})
	return nil
}

func cliKeymap(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	loadSettingsQuiet()
	km := loadKeymap(keymapPath())
	type row struct {
		Key    string         `json:"key"`
		Action string         `json:"action"`
		Args   map[string]any `json:"args,omitempty"`
		Source string         `json:"source"`
	}
	var rows []row
	for _, b := range km.bindings {
		rows = append(rows, row{b.spec, b.action, b.args, b.source})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Action < rows[j].Action })
	output(jsonOut, map[string]any{"path": km.path, "bindings": rows, "problems": km.problems}, func() {
		w := table()
		fmt.Fprintln(w, "KEY\tACTION\tARGS\tSOURCE")
		for _, r := range rows {
			args := "-"
			if len(r.Args) > 0 {
				b, _ := json.Marshal(r.Args)
				args = string(b)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Key, r.Action, args, r.Source)
		}
		w.Flush()
		for _, p := range km.problems {
			fmt.Fprintln(os.Stderr, p)
		}
	})
	return nil
}

func cliConfigCheck(args []string) error {
	f := newFlags()
	if err := f.parse(args); err != nil {
		return err
	}
	path := keymapPath()
	if len(f.args) > 0 {
		path = f.args[0]
	}
	loadSettingsQuiet()
	km := loadKeymap(path)
	exists := "no"
	if km.exists {
		exists = "yes"
	}
	user := 0
	for _, b := range km.bindings {
		if b.source != "default" {
			user++
		}
	}
	fmt.Printf("path: %s\nexists: %s\npreset: %s\nbindings: %d (%d from the file)\n", path, exists, presetOf(path), len(km.bindings), user)
	bad := false
	for _, p := range km.problems {
		fmt.Println(p)
		bad = bad || strings.HasPrefix(p, "error")
	}
	if bad {
		return errors.New("the keymap has errors")
	}
	return nil
}

func presetOf(path string) string {
	var f keymapFile
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &f) // config check reports a bad file
	}
	if f.Preset != "" {
		return f.Preset
	}
	return defaultPreset() + " (default)"
}

func cliConfigReload(args []string) error {
	var res json.RawMessage
	if err := do("client.do", &rex.Args{Action: "client.config.reload", Client: opts.client}, &res); err != nil {
		var re *rex.Error
		if errors.As(err, &re) && re.Code == "no_client" {
			fmt.Println("no app is running: it reads the keymap as it starts")
			return nil
		}
		return err
	}
	return cliConfigCheck(nil)
}

func cliServerStatus(args []string) error {
	var jsonOut bool
	if err := newFlags().boolean("json", &jsonOut).parse(args); err != nil {
		return err
	}
	c, err := rex.Dial()
	if err != nil {
		output(jsonOut, map[string]any{"running": false, "socket": rex.SocketPath()}, func() {
			fmt.Printf("not running (socket %s)\n", rex.SocketPath())
		})
		return &cliError{exitNoServer, errors.New("the server is not running")}
	}
	defer c.Close()
	h, err := c.Hello()
	if err != nil {
		return err
	}
	m, _ := c.Model()
	n := 0
	if m != nil {
		n = len(m.Sessions)
	}
	v := map[string]any{"running": true, "pid": h.PID, "started": h.Started, "protocol": h.Version,
		"socket": rex.SocketPath(), "dir": rex.Dir(), "exe": h.Exe, "sessions": n}
	output(jsonOut, v, func() {
		fmt.Printf("running, pid %d, up %s\nsocket   %s\nstate    %s\nsessions %d\nexe      %s\n",
			h.PID, roundDur(time.Since(h.Started)), rex.SocketPath(), rex.Dir(), n, h.Exe)
	})
	return nil
}

func cliServerStop(args []string) error {
	var forget bool
	if err := newFlags().boolean("forget", &forget).parse(args); err != nil {
		return err
	}
	c, err := rex.Dial()
	if err != nil {
		return &cliError{exitNoServer, errors.New("the server is not running")}
	}
	defer c.Close()
	if forget {
		return c.ShutdownForget()
	}
	return c.Shutdown()
}

func cliServerStart(args []string) error {
	c, err := rex.Connect()
	if err != nil {
		return &cliError{exitNoServer, err}
	}
	c.Close()
	return nil
}

func cliServerRun(args []string) error { return rex.Serve() }

func cliServerLogs(args []string) error {
	b, err := os.ReadFile(filepath.Join(rex.Dir(), "server.log"))
	if err != nil {
		return err
	}
	os.Stdout.Write(b)
	return nil
}

func cliCompletion(args []string) error {
	if len(args) != 1 {
		return usagef("usage: rex completion bash|zsh|fish")
	}
	words := strings.Join(slices.DeleteFunc(slices.Clone(cliCommands), func(s string) bool { return strings.HasPrefix(s, "-") }), " ")
	switch args[0] {
	case "bash":
		fmt.Printf("complete -W %q rex\n", words)
	case "zsh":
		fmt.Printf("#compdef rex\n_arguments '1: :(%s)'\n", words)
	case "fish":
		fmt.Printf("complete -c rex -f -n '__fish_use_subcommand' -a %q\n", words)
	default:
		return usagef("rex completion bash|zsh|fish")
	}
	return nil
}

// cliStatus reports a program's status for a block, as OSC 7501 would
// from its terminal, or, with no state, prints the block's records.
func cliStatus(args []string) error {
	var a rex.Args
	var app, msg, kind, progress, title, id string
	var jsonOut bool
	f := newFlags().targets(&a).str("app", &app).str("m,msg", &msg).str("kind", &kind).
		str("progress", &progress).str("title", &title).str("id", &id).boolean("json", &jsonOut)
	if err := f.parse(args); err != nil {
		return err
	}
	if len(f.args) == 0 {
		var res struct {
			Block   string             `json:"block_id"`
			Records []rex.StatusRecord `json:"records"`
		}
		if err := do("status.list", &a, &res); err != nil {
			return err
		}
		output(jsonOut, res, func() {
			if len(res.Records) == 0 {
				fmt.Println("no status")
			}
			for _, r := range res.Records {
				fmt.Println(statusLine(r))
			}
		})
		return nil
	}
	if len(f.args) != 1 {
		return usagef("usage: rex status [STATE] [--app APP] [--msg TEXT] [--kind KIND] [--progress PCT] [--title TEXT] [--id ID]")
	}
	a.Data = statusReport(f.args[0], id, kind, progress, app, title, msg)
	_, err := resolveIDs("status.report", &a)
	return err
}

// statusReport writes the body of an OSC 7501 report.
func statusReport(state, id, kind, progress, app, title, msg string) string {
	parts := []string{"state=" + state}
	add := func(k, v string) {
		if v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	add("id", id)
	add("kind", kind)
	add("progress", progress)
	add("app", app)
	if title != "" {
		add("title", base64.StdEncoding.EncodeToString([]byte(title)))
	}
	if msg != "" {
		add("msg", base64.StdEncoding.EncodeToString([]byte(msg)))
	}
	return strings.Join(parts, ":")
}

// statusLine writes a record for people.
func statusLine(r rex.StatusRecord) string {
	var b strings.Builder
	if r.ID != "" {
		b.WriteString(r.ID + "  ")
	}
	b.WriteString(r.State)
	if r.Kind != "" {
		b.WriteString(" (" + r.Kind + ")")
	}
	if r.Progress >= 0 {
		fmt.Fprintf(&b, " %d%%", r.Progress)
	}
	if r.App != "" {
		b.WriteString("  " + r.App)
	}
	if r.Title != "" {
		b.WriteString("  " + r.Title)
	}
	if r.Msg != "" {
		b.WriteString(": " + r.Msg)
	}
	b.WriteString("  " + ago(r.Updated))
	return b.String()
}
