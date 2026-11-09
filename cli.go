package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"os/user"
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
	// exitInterrupted is the status of a command an interrupt ended, as
	// Ctrl+C: 128 and SIGINT's number, as a shell has it.
	exitInterrupted = 130
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
		case "server_gone":
			return exitNoServer
		}
	}
	return exitFailed
}

// The commands.

// cliCmd is a command of rex, declared in one place: its words, its
// help, its flags and what it does. A group, as "session", has commands
// of its own.
type cliCmd struct {
	name    string    // its word: "split", or "new" in "rex session new"
	names   []string  // other words for it in the same place: "list" for "ls"
	use     string    // its arguments in the synopsis: "[LABEL] [-- COMMAND...]"
	short   string    // what it does, in a line of the overview
	help    string    // what it does, written by hand; the flags are listed from their declarations
	section string    // the overview's section that lists it; none: not listed
	sub     []*cliCmd // a group's commands
	of      *cliCmd   // the command this one is another name of, elsewhere: "capture" for "block capture"
	raw     func([]string) error
	bind    func() (*flagSet, func(*call) error)
	path    string // "session new", filled in from the tree
}

// cmd declares a command whose flags define declares in a T, and that
// run runs with them.
func cmd[T any](c cliCmd, define func(*T, *flagSet), run func(*T, *call) error) *cliCmd {
	c.bind = func() (*flagSet, func(*call) error) {
		o, f := new(T), newFlagSet("flag")
		if define != nil {
			define(o, f)
		}
		return f, func(cl *call) error { return run(o, cl) }
	}
	return &c
}

// rawCmd declares a command that reads its arguments itself: no flags.
func rawCmd(c cliCmd, run func([]string) error) *cliCmd {
	c.raw = run
	return &c
}

// group declares a command of commands.
func group(c cliCmd, sub ...*cliCmd) *cliCmd {
	c.sub = sub
	return &c
}

// alias is a command under another path, as "rex capture".
func alias(name, section string, of *cliCmd) *cliCmd {
	c := *of
	c.name, c.names, c.section, c.of = name, nil, section, of
	return &c
}

// flags returns the declarations of a command's flags.
func (c *cliCmd) flags() []*flagDef {
	if c.bind == nil {
		return nil
	}
	f, _ := c.bind()
	return f.flags
}

func (c *cliCmd) find(word string) *cliCmd {
	for _, s := range c.sub {
		if s.name == word || slices.Contains(s.names, word) {
			return s
		}
	}
	return nil
}

// The overview's sections, in order.
const (
	secCommon     = "Common commands"
	secLayout     = "Layout"
	secResources  = "Resources"
	secAutomation = "Automation"
	secServer     = "Server"
)

var sections = []string{secCommon, secLayout, secResources, secAutomation, secServer}

// rexCommands is rex's tree of commands, under a root without a name.
var rexCommands *cliCmd

func init() {
	// The commands of groups that are also commands of their own.
	sessionLs := cmd(cliCmd{name: "ls", names: []string{"list"}, short: "list sessions"}, listFlags, cliSessionLs)
	sessionNew := cmd(cliCmd{name: "new", use: "[LABEL] [-- COMMAND...]", short: "create a session",
		help: "Creates a session, after the others, with one window of one block, and prints its ID."},
		func(o *newOpts, f *flagSet) {
			createFlags(f, &o.create, "the directory rex runs in")
			f.str(&o.a.WindowLabel, "window", "", "a `LABEL` for its window")
			jsonFlag(f, &o.json)
		}, cliSessionNew)
	sessionKill := cmd[struct{}](cliCmd{name: "kill", use: "SESSION...", short: "end sessions"}, nil, cliSessionKill)
	windowNew := cmd(cliCmd{name: "new", use: "[LABEL] [-- COMMAND...]", short: "open a window",
		help: "Opens a window of one block in the session, and prints its ID. GoRex's first new-tab took the\ncommand without --: two arguments or more are the command."},
		func(o *newOpts, f *flagSet) {
			createFlags(f, &o.create, "with a command, the directory rex runs in; else the focused block's")
			f.str(&o.a.Session, "s,session", "", "the `SESSION` to open it in")
			f.boolean(&o.focus, "focus", true, "make it the active window; --focus=false leaves the focus where it is")
			jsonFlag(f, &o.json)
		}, cliWindowNew)
	blockSend := cmd(cliCmd{name: "send", use: "[TEXT...]", short: "type text into a terminal (stdin without TEXT)",
		help: "Types TEXT, its words joined by spaces, into a terminal; without TEXT, what comes on standard input."},
		func(a *rex.Args, f *flagSet) { targetFlags(f, a, "swb") }, cliSend)
	blockSendKey := cmd(cliCmd{name: "send-key", use: "KEY...", short: "press keys: enter, ctrl+c, shift+tab, up…", help: sendKeyHelp},
		func(o *sendKeyOpts, f *flagSet) {
			targetFlags(f, &o.a, "swb")
			f.str(&o.action, "action", "", "press, or release, which sends nothing: Rex's key `ACTION`")
		}, cliSendKey)
	blockCapture := cmd(cliCmd{name: "capture", short: "print a terminal's screen and scrollback", help: captureHelp},
		func(o *captureOpts, f *flagSet) {
			targetFlags(f, &o.a, "swb")
			f.choice(&o.a.Format, "format", "text", []string{"text", "vt"}, "the `FORMAT`: text, or vt to keep the escape sequences")
			f.boolean(&o.a.Trim, "trim", false, "drop trailing blank lines")
			f.boolean(&o.a.Unwrap, "unwrap", false, "join the rows of a line the terminal wrapped")
		}, cliCapture)
	blockZoom := cmd(cliCmd{name: "zoom", use: "[BLOCK]", short: "zoom a block to fill its window (--on, --off)",
		help: "Zooms a block to fill its window, or ends its zoom: without --on or --off, it toggles."},
		func(o *zoomOpts, f *flagSet) {
			targetFlags(f, &o.a, "swb")
			f.boolean(&o.on, "on", false, "zoom it")
			f.boolean(&o.off, "off", false, "end its zoom")
		}, cliZoom)
	blockMove := cmd(cliCmd{name: "move", use: "[BLOCK] DIRECTION ANCHOR", short: "move a block next to another",
		help: "Takes a block out of its window and puts it beside ANCHOR, toward DIRECTION: right, left,\nabove or below."},
		func(o *moveOpts, f *flagSet) {
			targetFlags(f, &o.a, "sw")
			f.percent(&o.ratio, "ratio", 50, pctRange{1, 99, true},
				"the block's share of its new split, in percent: `PCT`")
		}, cliMove)
	blockSwap := cmd(cliCmd{name: "swap", use: "BLOCK OTHER", short: "swap two blocks"},
		func(a *rex.Args, f *flagSet) { targetFlags(f, a, "sw") }, cliSwap)
	blockClose := cmd(cliCmd{name: "close", use: "[BLOCK...]", short: "close blocks",
		help: "Closes blocks; without BLOCK, the block rex runs in, as GoRex's first close did."},
		func(a *rex.Args, f *flagSet) { targetFlags(f, a, "sw") }, cliBlockClose)
	skillFlagsCmd := func(name, use, short string) *cliCmd {
		return cmd(cliCmd{name: name, use: use, short: short}, skillFlags, func(o *skillCLI, c *call) error { return cliSkill(name, o) })
	}

	rexCommands = group(cliCmd{},
		alias("ls", secCommon, sessionLs),
		alias("new", secCommon, sessionNew),
		cmd(cliCmd{name: "run", use: "-- COMMAND...", short: "run a command in a new window, or beside a block (--split)", section: secCommon, help: placeHelp},
			func(o *placeOpts, f *flagSet) { placeFlags(o, f, false) }, func(o *placeOpts, c *call) error { return cliPlace(o, c, false) }),
		cmd(cliCmd{name: "split", use: "[-- COMMAND...]", short: "split a block (yours, by default)", section: secCommon, help: placeHelp},
			func(o *placeOpts, f *flagSet) { placeFlags(o, f, true) }, func(o *placeOpts, c *call) error { return cliPlace(o, c, true) }),
		alias("send", secCommon, blockSend),
		alias("send-key", secCommon, blockSendKey),
		alias("capture", secCommon, blockCapture),
		alias("kill", secCommon, sessionKill),

		cmd(cliCmd{name: "focus", use: "TARGET|DIRECTION", short: "focus a block or window, or left/right/up/down/next/prev", section: secLayout},
			func(a *rex.Args, f *flagSet) {
				targetFlags(f, a, "sw")
				f.boolean(&a.Wrap, "wrap", false, "from the last, go on to the first")
			}, cliFocus),
		alias("zoom", secLayout, blockZoom),
		alias("move", secLayout, blockMove),
		alias("swap", secLayout, blockSwap),
		cmd(cliCmd{name: "resize", use: "[BLOCK] DIRECTION", short: "move a block's divider (--amount PCT)", section: secLayout,
			help: "Moves the divider nearest the block on DIRECTION's axis toward DIRECTION."},
			func(o *resizeOpts, f *flagSet) {
				targetFlags(f, &o.a, "swb")
				f.percent(&o.amount, "amount", 5, pctRange{0, math.Inf(1), false},
					"how far to move it, `PCT` percent of the split")
				f.str(&o.a.Direction, "direction", "", "the `DIRECTION`, when no argument gives it")
			}, cliResize),
		alias("close", secLayout, blockClose),

		group(cliCmd{name: "session", short: "sessions", section: secResources},
			sessionNew, sessionLs,
			cmd(cliCmd{name: "inspect", use: "[SESSION]", short: "show a session and its windows"},
				func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliSessionInspect),
			cmd[struct{}](cliCmd{name: "rename", use: "[SESSION] LABEL", short: "rename a session"}, nil, cliSessionRename),
			cmd(cliCmd{name: "move", use: "[SESSION]", short: "move a session in the order of sessions"},
				func(o *orderOpts, f *flagSet) { orderFlags(f, o) }, cliSessionMove),
			sessionKill),
		group(cliCmd{name: "window", short: "windows, the app's tabs", section: secResources},
			windowNew,
			cmd(cliCmd{name: "ls", names: []string{"list"}, short: "list a session's windows"},
				func(o *lsOpts, f *flagSet) {
					targetFlags(f, &o.a, "s")
					f.boolean(&o.all, "A,all", false, "every session's")
					listFlags(&o.listOpts, f)
				}, cliWindowLs),
			cmd(cliCmd{name: "inspect", use: "[WINDOW]", short: "show a window and its blocks"},
				func(o *inspectOpts, f *flagSet) { targetFlags(f, &o.a, "s"); jsonFlag(f, &o.json) }, cliWindowInspect),
			cmd(cliCmd{name: "focus", use: "[WINDOW]", short: "make a window the active one"},
				func(a *rex.Args, f *flagSet) {
					targetFlags(f, a, "s")
					f.boolean(&a.Wrap, "wrap", false, "from the last, go on to the first")
				}, cliWindowFocus),
			cmd(cliCmd{name: "move", use: "[WINDOW]", short: "move a window in its session"},
				func(o *orderOpts, f *flagSet) { targetFlags(f, &o.a, "s"); orderFlags(f, o) }, cliWindowMove),
			cmd(cliCmd{name: "rename", use: "[WINDOW] LABEL", short: "rename a window"},
				func(a *rex.Args, f *flagSet) { targetFlags(f, a, "s") }, cliWindowRename),
			cmd(cliCmd{name: "close", use: "WINDOW...", short: "close windows"},
				func(a *rex.Args, f *flagSet) { targetFlags(f, a, "s") }, cliWindowClose)),
		group(cliCmd{name: "block", short: "blocks, each a terminal: the app's panes", section: secResources},
			cmd(cliCmd{name: "ls", names: []string{"list"}, short: "list blocks"},
				func(o *lsOpts, f *flagSet) {
					targetFlags(f, &o.a, "sw")
					f.boolean(&o.all, "A,all", false, "every session's")
					listFlags(&o.listOpts, f)
				}, cliBlockLs),
			cmd(cliCmd{name: "inspect", use: "[BLOCK]", short: "show a block"},
				func(o *inspectOpts, f *flagSet) { targetFlags(f, &o.a, "swb"); jsonFlag(f, &o.json) }, cliBlockInspect),
			cmd(cliCmd{name: "focus", use: "[BLOCK]", short: "focus a block"},
				func(a *rex.Args, f *flagSet) { targetFlags(f, a, "swb") }, cliBlockFocus),
			blockSend, blockSendKey, blockCapture, blockZoom, blockMove, blockSwap,
			cmd(cliCmd{name: "rename", use: "[BLOCK] LABEL", short: "rename a block"},
				func(a *rex.Args, f *flagSet) { targetFlags(f, a, "sw") }, cliBlockRename),
			blockClose,
			cmd(cliCmd{name: "types", short: "list the kinds of blocks"}, func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliBlockTypes),
			rawCmd(cliCmd{name: "new", short: "refused: rex run or rex split start a block"}, cliBlockNew)),
		group(cliCmd{name: "client", short: "the apps and commands connected", section: secResources},
			cmd(cliCmd{name: "ls", names: []string{"list"}, short: "list the clients"},
				func(o *clientLsOpts, f *flagSet) {
					f.boolean(&o.all, "all", false, "rex's commands too")
					listFlags(&o.listOpts, f)
				}, cliClientLs),
			cmd(cliCmd{name: "inspect", use: "[CLIENT]", short: "print an app and the actions it offers",
				help: "CLIENT is an app's ID, name, position or end of ID. Without it, -C or REX_CLIENT names\nthe app, else the app when exactly one is connected, as rex do finds it."},
				func(o *clientInspectOpts, f *flagSet) { jsonFlag(f, &o.json) }, cliClientInspect)),
		cmd(cliCmd{name: "list", short: "the whole tree of sessions, windows and blocks", section: secResources},
			func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliTree),

		cmd(cliCmd{name: "events", short: "stream what happens", section: secAutomation, help: eventsHelp},
			func(o *eventsOpts, f *flagSet) {
				targetFlags(f, &o.a, "s")
				f.boolean(&o.a.All, "all", false, "every event")
				jsonFlag(f, &o.json)
			}, cliEvents),
		cmd(cliCmd{name: "wait", use: "TARGET", short: "wait for a block's program (exits with its status), a window or a session", section: secAutomation, help: waitHelp},
			func(o *waitOpts, f *flagSet) {
				targetFlags(f, &o.a, "sw")
				f.duration(&o.limit, "for", 0, "give up after `DURATION`, with status 1").defaults("no limit")
				jsonFlag(f, &o.json)
			}, cliWait),
		cmd(cliCmd{name: "status", use: "[STATE]", short: "report a program's status (OSC 7501), or show a block's", section: secAutomation, help: statusHelp},
			func(o *statusOpts, f *flagSet) {
				targetFlags(f, &o.a, "swb")
				f.str(&o.app, "app", "", "the `APP` that reports")
				f.str(&o.msg, "m,msg", "", "a message: `TEXT`")
				f.str(&o.kind, "kind", "", "what it waits for, the `KIND`: permission, question or auth")
				f.str(&o.progress, "progress", "", "how far it is, `PCT` percent")
				f.str(&o.title, "title", "", "a title: `TEXT`")
				f.str(&o.id, "id", "", "the `ID` of the record, for a program that reports several")
				jsonFlag(f, &o.json)
			}, cliStatus),
		cmd(cliCmd{name: "do", use: "ACTION [KEY=VALUE...]", short: "perform an action of the app, as its keys do", section: secAutomation, help: doHelp},
			func(o *doOpts, f *flagSet) {
				targetFlags(f, &o.a, "swb")
				f.str(&o.args, "args", "", "the action's arguments as a `JSON` object; - reads stdin, @FILE a file")
				f.str(&o.eval, "e,eval", "", "Rex's Lua `CODE`: refused")
				f.str(&o.action, "a,action", "", "Rex's `ACTION`").ignore()
				jsonFlag(f, &o.json)
				var ignored string
				var ignoredBool bool
				f.str(&ignored, "origin", "", "Rex's `ORIGIN`").ignore()
				f.str(&ignored, "for", "", "Rex's `DURATION`").ignore()
				f.boolean(&ignoredBool, "all", false, "Rex's").ignore()
				f.boolean(&ignoredBool, "fail-fast", false, "Rex's").ignore()
			}, cliDo),
		cmd(cliCmd{name: "actions", short: "list the app's actions", section: secAutomation},
			func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliActions),
		cmd(cliCmd{name: "keymap", short: "list the key bindings in effect", section: secAutomation, help: keymapHelp},
			func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliKeymap),
		group(cliCmd{name: "config", short: "check, or have the app reload, keymap.json", section: secAutomation},
			cmd(cliCmd{name: "check", use: "[PATH]", short: "check keymap.json, or the file at PATH"},
				func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliConfigCheck),
			cmd(cliCmd{name: "reload", short: "have the app read keymap.json again"},
				func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliConfigReload)),
		group(cliCmd{name: "skill", short: "set up agents (Claude Code, Codex…) to use rex", section: secAutomation, help: skillHelp},
			skillFlagsCmd("setup", "", "install, or update, the skill and the hooks"),
			skillFlagsCmd("status", "", "tell what is installed"),
			skillFlagsCmd("remove", "", "remove what rex installed"),
			rawCmd(cliCmd{name: "show", short: "print the skill"}, cliSkillShow),
			rawCmd(cliCmd{name: "hook", use: "[AGENT] [EVENT]", short: "the hook agents run"}, cliSkillHook)),

		group(cliCmd{name: "server", short: "the server that keeps the sessions", section: secServer},
			cmd(cliCmd{name: "status", short: "tell whether the server runs"}, func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliServerStatus),
			cmd(cliCmd{name: "stop", short: "stop the server and its programs", help: stopHelp},
				func(o *stopOpts, f *flagSet) {
					f.boolean(&o.discard, "discard,forget", false, "delete the saved sessions once the server has stopped; --forget is GoRex's first name")
					f.boolean(&o.ifIdle, "if-idle", false, "stop only if the server has no sessions and no other client")
				}, cliServerStop),
			cmd(cliCmd{name: "logs", short: "print the server's log"},
				func(o *logsOpts, f *flagSet) {
					f.boolean(&o.follow, "follow", false, "print what the server adds to it, until the server stops")
					f.position(&o.lines, "n,lines", "print the newest `N` lines").defaults("200")
					f.boolean(&o.all, "all", false, "print the whole log")
				}, cliServerLogs),
			rawCmd(cliCmd{name: "start", short: "start the server, unless one runs"}, cliServerStart),
			rawCmd(cliCmd{name: "run", short: "run the server in the foreground"}, cliServerRun)),
		cmd(cliCmd{name: "version", short: "print rex's version, and the server's", section: secServer},
			func(o *bool, f *flagSet) { jsonFlag(f, o) }, cliVersion),
		rawCmd(cliCmd{name: "completion", use: "bash|zsh|fish", short: "print a script that completes rex's commands", section: secServer}, cliCompletion),

		// GoRex's first commands.
		alias("new-tab", "", windowNew),
		rawCmd(cliCmd{name: "send-keys", use: "[BLOCK] TEXT", short: "GoRex's first send: <CR>, <Esc>, \\n, \\r and \\e are keys"}, cliSendKeysCompat),
	)
	setPaths(rexCommands, "")
}

func setPaths(c *cliCmd, prefix string) {
	for _, s := range c.sub {
		s.path = strings.TrimSpace(prefix + " " + s.name)
		setPaths(s, s.path)
	}
}

// The flags commands share.

// targetFlags declares the targets: s for -s, w for -w, b for -b.
func targetFlags(f *flagSet, a *rex.Args, which string) {
	for _, t := range which {
		switch t {
		case 's':
			f.str(&a.Session, "s,session", "", "the `SESSION` to act in")
		case 'w':
			f.str(&a.Window, "w,window", "", "the `WINDOW` to act in")
		case 'b':
			f.str(&a.Block, "b,block", "", "the `BLOCK` to act on").defaults("yours, else the window's focused block")
		}
	}
}

func jsonFlag(f *flagSet, p *bool) { f.boolean(p, "json", false, "print JSON") }

// listOpts are the flags of a listing.
type listOpts struct{ json, quiet, short bool }

func listFlags(o *listOpts, f *flagSet) {
	jsonFlag(f, &o.json)
	f.boolean(&o.quiet, "q,quiet", false, "print the IDs alone, one a line")
	f.boolean(&o.short, "short", true, "short IDs; --short=false prints full ones")
}

type lsOpts struct {
	listOpts
	a   rex.Args
	all bool
}

type inspectOpts struct {
	a    rex.Args
	json bool
}

// createFlags are the flags of commands that start a terminal; dir is
// where it starts without --cwd.
func createFlags(f *flagSet, o *rex.CreateOptions, dir string) {
	f.str(&o.Dir, "c,cwd,d,dir", "", "start in `DIR`; ~ is your home, ~NAME that user's").defaults(dir)
	f.boolean(&o.KeepOpen, "keep-open", false, "keep the terminal after its command exits")
	f.str(&o.Shell, "shell", "", "run the command in `SHELL`'s login environment; none: directly").defaults("your login shell")
	f.boolean(&o.Interactive, "interactive", false, "also read the shell's interactive startup files")
}

type newOpts struct {
	create rex.CreateOptions
	a      rex.Args
	json   bool
	focus  bool
}

// orderOpts are the flags of a command that moves a session or window.
type orderOpts struct {
	a     rex.Args
	index int
}

func orderFlags(f *flagSet, o *orderOpts) {
	f.str(&o.a.Before, "before", "", "put it before `TARGET`")
	f.str(&o.a.After, "after", "", "put it after `TARGET`")
	f.position(&o.index, "index", "put it at position `N`, from 1")
}

// The global options, before the command.

// global are the options every command takes.
type global struct {
	autostart bool
	timeout   time.Duration
	client    string
}

var opts global

func init() { globalFlags(&opts) }

// globalFlags declares the global options into o.
func globalFlags(o *global) *flagSet {
	f := newFlagSet("option")
	f.add("S,server", "Rex's remote server: refused, GoRex's server runs on this machine", funcValue(func(string) error {
		return usagef("connecting to other servers is not supported: GoRex's server runs on this machine")
	}), true)
	f.str(&o.client, "C,client", "", "the app, `CLIENT`, that rex do asks").defaults("REX_CLIENT")
	f.duration(&o.timeout, "timeout", 10*time.Second, "wait up to `DURATION` for an answer")
	f.boolean(&o.autostart, "autostart", true, "start a server when none runs; --autostart=false does not")
	var color string
	f.str(&color, "color", "", "Rex's `WHEN`").ignore()
	f.add("v,verbose,vv", "Rex's", funcValue(func(string) error { return nil }), true).ignore()
	return f
}

// isCLICommand tells whether a word is a command of rex.
func isCLICommand(s string) bool {
	return s == "help" || s == "--help" || s == "-h" || rexCommands.find(s) != nil
}

// isCLI tells whether GoRex's arguments are a command of rex, after
// rex's global options: then GoRex runs as rex, not as the app. A global
// option with a bad value runs rex too, which says what is wrong.
func isCLI(args []string) bool {
	var g global
	f := globalFlags(&g)
	rest, err := f.parseLeading(args)
	if err != nil {
		name, _, _ := strings.Cut(strings.TrimLeft(args[0], "-"), "=")
		return f.names[name] != nil
	}
	return len(rest) > 0 && isCLICommand(rest[0])
}

// runCLI runs rex with its arguments and returns the exit status.
func runCLI(args []string) int {
	args, err := globalFlags(&opts).parseLeading(args)
	if err != nil {
		fatalf("%v", err)
		return exitUsage
	}
	if len(args) == 0 {
		printHelp("")
		return exitOK
	}
	if c := os.Getenv("REX_CLIENT"); opts.client == "" {
		opts.client = c
	}
	start := time.Now()
	stop := onInterrupt(func() {
		wideCommand(args, exitInterrupted, errors.New("interrupted"), start)
		os.Exit(exitInterrupted)
	})
	err = dispatch(args[0], args[1:])
	stop()
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

// onInterrupt runs f when the command is interrupted, until stop.
func onInterrupt(f func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			f()
		case <-done:
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}

// wideCommand records a rex command that ran: which, from where (a GoRex
// terminal, an agent), how it ended and how long it took. Of its
// arguments, it keeps the command's names, as the tree declares them,
// and the flags' names, not the values or the other arguments: those
// may be text typed into terminals.
func wideCommand(args []string, code int, err error, start time.Time) {
	switch args[0] {
	case "help", "--help", "-h", "completion", "__complete":
		return
	}
	// Of the flags, only the names a command declares: a word that only
	// looks like a flag may be text the user typed (B-8).
	declared := globalFlags(&global{}).names
	if c := commandNode(args); c != nil && c.bind != nil {
		fs, _ := c.bind()
		declared = fs.names
	}
	var flags []string
	unknown := 0
	for _, a := range args {
		if a == "--" {
			break // the command it runs
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			name, _, _ := strings.Cut(strings.TrimLeft(a, "-"), "=")
			if d := declared[name]; d != nil {
				flags = append(flags, d.name())
			} else {
				unknown++
			}
		}
	}
	w := rex.W{
		"command":     commandPath(args),
		"exit_code":   code,
		"ok":          code == exitOK,
		"duration_ms": float64(time.Since(start).Microseconds()) / 1000,
		"n_args":      len(args) - 1,
		"in_rex":      os.Getenv("REX_BLOCK") != "",
	}
	if len(flags) > 0 {
		w["flags"] = strings.Join(flags, ",")
	}
	if unknown > 0 {
		w["n_unknown_flags"] = unknown
	}
	// A usage error quotes the command line; its text stays out (B-8).
	switch {
	case err == nil || errors.Is(err, errQuiet):
	case code == exitUsage:
		w["error"] = "usage"
	default:
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

// asksHelp tells whether a command's arguments ask for its help: --help
// or -h, before "--".
func asksHelp(args []string) bool {
	for _, a := range args {
		switch a {
		case "--":
			return false
		case "--help", "-h":
			return true
		}
	}
	return false
}

// commandPath is the path in the tree of the command that args name, as
// dispatch finds it: "session new", or "session" for a group and a word
// it does not have; empty for a word that is not a command.
func commandPath(args []string) string {
	if c := commandNode(args); c != nil {
		return c.path
	}
	return ""
}

// commandNode is the declared command the words name, or nil.
func commandNode(args []string) *cliCmd {
	c := rexCommands.find(args[0])
	if c == nil {
		return nil
	}
	if c.sub != nil && len(args) > 1 {
		if s := c.find(args[1]); s != nil {
			return s
		}
	}
	return c
}

// dispatch runs the command that word names, with its arguments.
func dispatch(word string, args []string) error {
	switch word {
	case "help", "--help", "-h":
		printHelp(strings.Join(args, " "))
		return nil
	}
	c := rexCommands.find(word)
	if c == nil {
		return usagef("unknown command %q: run \"rex help\"", word)
	}
	if c.sub != nil {
		if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
			printHelp(c.path)
			return nil
		}
		s := c.find(args[0])
		if s == nil {
			return usagef("unknown command \"%s %s\": run \"rex help %s\"", word, args[0], word)
		}
		c, args = s, args[1:]
	}
	if asksHelp(args) {
		fmt.Print(commandHelp(c))
		return nil
	}
	if c.raw != nil {
		return c.raw(args)
	}
	f, run := c.bind()
	cl, err := f.parse(args)
	if err != nil {
		return err
	}
	return run(cl)
}

// The connection.

// connect connects to the server: it starts one when none runs, unless
// --autostart=false.
func connect() (*rex.Client, error) {
	if err := rex.ServerEnvError(); err != nil {
		return nil, &cliError{exitUsage, err}
	}
	var c *rex.Client
	var err error
	if opts.autostart {
		c, err = rex.ConnectFor(opts.timeout)
	} else {
		c, err = rex.DialFor(opts.timeout)
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

func cliVersion(o *bool, _ *call) error {
	jsonOut := *o
	v := map[string]any{"version": rex.Version, "protocol": rex.ProtocolVersion, "name": "GoRex"}
	if c, err := rex.DialFor(opts.timeout); err == nil {
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

func absPath(p string) (string, error) { return filepath.Abs(p) }

// fixDir makes --cwd absolute: ~ is the user's home, ~NAME that user's,
// and a relative path starts from the directory rex runs in.
func fixDir(o *rex.CreateOptions) {
	if o.Dir != "" {
		if home, rest, ok := homeOf(o.Dir); ok {
			o.Dir = filepath.Join(home, rest)
		}
		if abs, err := absPath(o.Dir); err == nil {
			o.Dir = abs
		}
	}
}

// homeOf splits a path that starts with ~ or ~NAME into the home it
// names and the rest; ok is false for another path, or a user that does
// not exist, which stays as it is, as a shell leaves it.
func homeOf(p string) (home, rest string, ok bool) {
	if !strings.HasPrefix(p, "~") {
		return "", "", false
	}
	name, rest, _ := strings.Cut(p[1:], "/")
	if name == "" {
		home, err := os.UserHomeDir()
		return home, rest, err == nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return "", "", false
	}
	return u.HomeDir, rest, true
}

func cliSessionNew(n *newOpts, f *call) error {
	o, a, jsonOut := &n.create, &n.a, n.json
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
	fixDir(o)
	a.Create = o
	var res map[string]any
	if err := do("session.new", a, &res); err != nil {
		return err
	}
	output(jsonOut, res, func() { fmt.Println(res["session_id"]) })
	return nil
}

func cliSessionLs(o *listOpts, _ *call) error {
	jsonOut, quiet, shortIDs := o.json, o.quiet, o.short
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

func cliSessionInspect(o *bool, f *call) error {
	var a rex.Args
	jsonOut := *o
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

func cliSessionRename(_ *struct{}, f *call) error {
	var a rex.Args
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

func cliSessionMove(o *orderOpts, f *call) error {
	a := &o.a
	if len(f.args) > 0 {
		a.Session = f.args[0]
	}
	if n := countSet(a.Before != "", a.After != "", o.index != 0); n != 1 {
		return usagef("exactly one of --before, --after or --index is needed")
	}
	a.Index = o.index
	_, err := resolveIDs("session.move", a)
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

func cliSessionKill(_ *struct{}, f *call) error {
	if len(f.args) == 0 {
		return usagef("usage: rex kill SESSION...")
	}
	return eachTarget(&rex.Args{}, f.args, "session", func(a *rex.Args, t string) { a.Session = t })
}

// eachTarget closes every target of a kind, or kills every session, once
// it resolved them all: when one does not resolve, nothing changes.
func eachTarget(base *rex.Args, targets []string, kind string, set func(*rex.Args, string)) error {
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	var ids []string
	for _, t := range targets {
		a := *base
		set(&a, t)
		var res map[string]any
		if err := c.Do(kind+".resolve", withContext(&a), &res); err != nil {
			return err
		}
		if id, _ := res[kind+"_id"].(string); !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	op := kind + ".close"
	if kind == "session" {
		op = "session.kill"
	}
	for _, id := range ids {
		a := *base
		set(&a, id)
		if err := c.Do(op, withContext(&a), nil); err != nil {
			return err
		}
	}
	return nil
}

func cliWindowNew(n *newOpts, f *call) error {
	o, a, jsonOut, focus := &n.create, &n.a, n.json, n.focus
	if len(f.args) > 1 {
		// GoRex's first new-tab took the command without "--".
		f.command = append(f.args, f.command...)
		f.args = nil
	}
	if len(f.args) == 1 {
		a.WindowLabel = f.args[0]
	}
	o.Command = f.command
	if o.Dir == "" {
		o.Dir, _ = os.Getwd()
	}
	fixDir(o)
	a.Create, a.NoFocus = o, !focus
	var res map[string]any
	if err := do("window.new", a, &res); err != nil {
		return err
	}
	output(jsonOut, res, func() { fmt.Println(res["window_id"]) })
	return nil
}

func cliWindowLs(o *lsOpts, _ *call) error {
	a, jsonOut, quiet, all, shortIDs := &o.a, o.json, o.quiet, o.all, o.short
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
		if m, ss, err = findSession(a); err != nil {
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

func cliWindowInspect(o *inspectOpts, f *call) error {
	a, jsonOut := &o.a, o.json
	if len(f.args) > 0 {
		a.Window = f.args[0]
	}
	m, ss, w, err := findWindow(a)
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

func cliWindowFocus(a *rex.Args, f *call) error {
	if len(f.args) > 0 {
		a.Target = f.args[0]
	}
	_, err := resolveIDs("window.focus", a)
	return err
}

func cliWindowMove(o *orderOpts, f *call) error {
	a := &o.a
	if len(f.args) > 0 {
		a.Window = f.args[0]
	}
	if n := countSet(a.Before != "", a.After != "", o.index != 0); n != 1 {
		return usagef("exactly one of --before, --after or --index is needed")
	}
	a.Index = o.index
	_, err := resolveIDs("window.move", a)
	return err
}

func cliWindowRename(a *rex.Args, f *call) error {
	switch len(f.args) {
	case 1:
		a.Label = f.args[0]
	case 2:
		a.Window, a.Label = f.args[0], f.args[1]
	default:
		return usagef("usage: rex window rename [WINDOW] LABEL")
	}
	_, err := resolveIDs("window.rename", a)
	return err
}

func cliWindowClose(a *rex.Args, f *call) error {
	if len(f.args) == 0 {
		return usagef("usage: rex window close WINDOW...")
	}
	return eachTarget(a, f.args, "window", func(a *rex.Args, t string) { a.Window = t })
}

// placeOpts are the flags of rex run and rex split.
type placeOpts struct {
	o                       rex.CreateOptions
	a                       rex.Args
	dir                     string
	ratio                   float64
	json, wait, down, right bool
	focus                   bool
}

func placeFlags(p *placeOpts, f *flagSet, split bool) {
	dir, side := "the directory rex runs in", ""
	sideUsage := "put the block beside the anchor, toward `DIRECTION`: right, left, above or below"
	if split {
		dir, side = "the anchor's", "right"
		sideUsage = "the `DIRECTION` of the new block: right, left, above or below"
	}
	createFlags(f, &p.o, dir)
	targetFlags(f, &p.a, "swb")
	d := f.str(&p.dir, "split", side, sideUsage)
	if !split {
		d.defaults("a new window")
	}
	f.percent(&p.ratio, "ratio", 50, pctRange{1, 99, true},
		"the new block's share of the split, in percent: `PCT`")
	f.str(&p.o.Label, "label", "", "a `LABEL` to target it by")
	f.boolean(&p.focus, "focus", true, "focus it; --focus=false leaves the focus where it is")
	jsonFlag(f, &p.json)
	f.boolean(&p.wait, "wait", false, "wait for the command, and exit with its status")
	// GoRex's first split's flags.
	f.str(&p.a.Block, "t,target", "", "GoRex's first --block `BLOCK`")
	f.boolean(&p.down, "v,vertical,down,below", false, "GoRex's first --split=below")
	f.boolean(&p.right, "horizontal,right,beside", false, "GoRex's first --split=right")
}

// cliPlace is "rex run" and "rex split": a terminal in a new window, or
// beside a block.
func cliPlace(p *placeOpts, f *call, split bool) error {
	o, a, dir, jsonOut, wait, down, right, focus := &p.o, &p.a, p.dir, p.json, p.wait, p.down, p.right, p.focus
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
	if o.Dir == "" && !split && dir == "" {
		// A new window starts where rex runs; a split, in its anchor's
		// directory.
		o.Dir, _ = os.Getwd()
	}
	fixDir(o)
	a.Ratio = float32(p.ratio / 100)
	a.Create, a.NoFocus, a.Direction = o, !focus, dir
	op := "block.split"
	if !split && dir == "" {
		op = "window.new"
		a.Block = ""
	}
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	var res map[string]any
	if err := c.DoFor(op, withContext(a), &res, opts.timeout); err != nil {
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

func cliBlockNew([]string) error {
	return usagef("GoRex's blocks are terminals: rex run or rex split start one")
}

func cliBlockTypes(o *bool, _ *call) error {
	jsonOut := *o
	types := []map[string]string{{"flavor": "com.superlogical.terminal.shell", "title": "Terminal"}}
	output(jsonOut, map[string]any{"types": types}, func() { fmt.Println("com.superlogical.terminal.shell  Terminal") })
	return nil
}

func cliSend(a *rex.Args, f *call) error {
	text := strings.Join(append(f.args, f.command...), " ")
	if len(f.args) == 0 && len(f.command) == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		text = string(b)
	}
	a.Data = text
	_, err := resolveIDs("send", a)
	return err
}

type sendKeyOpts struct {
	a      rex.Args
	action string
}

func cliSendKey(o *sendKeyOpts, f *call) error {
	a, action := &o.a, o.action
	if len(f.args) == 0 {
		return usagef("usage: rex send-key KEY...")
	}
	if action == "release" {
		return nil // a terminal sends nothing as a key is released
	}
	a.Keys = f.args
	_, err := resolveIDs("sendKey", a)
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

type captureOpts struct{ a rex.Args }

func cliCapture(o *captureOpts, _ *call) error {
	a := &o.a
	var res map[string]any
	if err := do("capture", a, &res); err != nil {
		return err
	}
	fmt.Print(res["text"])
	return nil
}

func cliFocus(a *rex.Args, f *call) error {
	if len(f.args) != 1 {
		return usagef("usage: rex focus TARGET|DIRECTION")
	}
	a.Target = f.args[0]
	_, err := resolveIDs("focus", a)
	return err
}

func cliBlockFocus(a *rex.Args, f *call) error {
	if len(f.args) > 0 {
		a.Block = f.args[0]
	}
	_, err := resolveIDs("block.focus", a)
	return err
}

type zoomOpts struct {
	a       rex.Args
	on, off bool
}

func cliZoom(o *zoomOpts, f *call) error {
	a, on, off := &o.a, o.on, o.off
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
	_, err := resolveIDs("block.zoom", a)
	return err
}

type moveOpts struct {
	a     rex.Args
	ratio float64
}

func cliMove(o *moveOpts, f *call) error {
	a := &o.a
	switch len(f.args) {
	case 2:
		a.Direction, a.Target = f.args[0], f.args[1]
	case 3:
		a.Block, a.Direction, a.Target = f.args[0], f.args[1], f.args[2]
	default:
		return usagef("usage: rex move [BLOCK] DIRECTION ANCHOR")
	}
	a.Ratio = float32(o.ratio / 100)
	_, err := resolveIDs("block.move", a)
	return err
}

func cliSwap(a *rex.Args, f *call) error {
	if len(f.args) != 2 {
		return usagef("usage: rex swap BLOCK OTHER")
	}
	a.Block, a.Target = f.args[0], f.args[1]
	_, err := resolveIDs("block.swap", a)
	return err
}

type resizeOpts struct {
	a      rex.Args
	amount float64
}

func cliResize(o *resizeOpts, f *call) error {
	a := &o.a
	switch len(f.args) {
	case 1:
		a.Direction = f.args[0]
	case 2:
		a.Block, a.Direction = f.args[0], f.args[1]
	}
	if a.Direction == "" {
		return usagef("usage: rex resize [BLOCK] DIRECTION [--amount PCT]")
	}
	a.Amount = float32(o.amount)
	_, err := resolveIDs("block.resize", a)
	return err
}

func cliBlockRename(a *rex.Args, f *call) error {
	switch len(f.args) {
	case 1:
		a.Label = f.args[0]
	case 2:
		a.Block, a.Label = f.args[0], f.args[1]
	default:
		return usagef("usage: rex block rename [BLOCK] LABEL")
	}
	_, err := resolveIDs("block.rename", a)
	return err
}

func cliBlockClose(a *rex.Args, f *call) error {
	if len(f.args) == 0 {
		// GoRex's first close closed the terminal it ran in.
		_, err := resolveIDs("block.close", a)
		return err
	}
	return eachTarget(a, f.args, "block", func(a *rex.Args, t string) { a.Block = t })
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

func cliBlockLs(o *lsOpts, _ *call) error {
	a, jsonOut, quiet, all, shortIDs := &o.a, o.json, o.quiet, o.all, o.short
	_, rows, err := blockRows()
	if err != nil {
		return err
	}
	if !all {
		// The session, or window, the command acts in.
		var keep func(r blockRow) bool
		if a.Window != "" {
			_, _, w, err := findWindow(a)
			if err != nil {
				return err
			}
			keep = func(r blockRow) bool { return r.Window == w.ID }
		} else {
			_, ss, err := findSession(a)
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

func cliBlockInspect(o *inspectOpts, f *call) error {
	a, jsonOut := &o.a, o.json
	if len(f.args) > 0 {
		a.Block = f.args[0]
	}
	var res map[string]any
	if err := do("block.resolve", a, &res); err != nil {
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
func cliTree(o *bool, _ *call) error {
	jsonOut := *o
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

type clientLsOpts struct {
	listOpts
	all bool
}

func cliClientLs(o *clientLsOpts, _ *call) error {
	jsonOut, all, full := o.json, o.all, !o.short
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
	if !o.quiet {
		fmt.Fprintln(w, "ID\tKIND\tNAME\tAGE\tVIEWING")
	}
	for _, cl := range cs {
		switch {
		case cl.Kind == "cli" && !all:
		case o.quiet:
			fmt.Fprintln(w, short(cl.ID, full))
		default:
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", short(cl.ID, full), cl.Kind, dash(cl.Name), ago(cl.Connected), short(cl.Viewing, full))
		}
	}
	return w.Flush()
}

type clientInspectOpts struct{ json bool }

// cliClientInspect prints the app that -C would reach, found by the
// server as rex do finds it, and the actions it offers.
func cliClientInspect(o *clientInspectOpts, c *call) error {
	name := opts.client
	switch len(c.args) {
	case 0:
	case 1:
		name = c.args[0]
	default:
		return usagef("usage: rex client inspect [CLIENT]")
	}
	var cl rex.ClientInfo
	if err := do("client.inspect", &rex.Args{Client: name}, &cl); err != nil {
		return err
	}
	actions := actionList()
	output(o.json, map[string]any{"client": cl, "actions": actions}, func() {
		fmt.Printf("%s  %s  %s  connected %s\n", cl.ID, cl.Kind, dash(cl.Name), ago(cl.Connected))
		w := table()
		fmt.Fprintln(w, "\nACTION\tTITLE\tARGS")
		for _, x := range actions {
			fmt.Fprintf(w, "%s\t%s\t%s\n", x.Name, x.Title, x.Args)
		}
		w.Flush()
	})
	return nil
}

type eventsOpts struct {
	a    rex.Args
	json bool
}

func cliEvents(o *eventsOpts, _ *call) error {
	a, jsonOut := o.a, o.json
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
			return &cliError{exitNoServer, rex.ErrServerGone}
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

type waitOpts struct {
	a     rex.Args
	limit time.Duration
	json  bool
}

func cliWait(o *waitOpts, f *call) error {
	a, d, jsonOut := &o.a, o.limit, o.json
	if len(f.args) != 1 {
		return usagef("usage: rex wait TARGET")
	}
	a.Target = f.args[0]
	c, err := connect()
	if err != nil {
		return err
	}
	defer c.Close()
	var res map[string]any
	if err := c.DoFor("wait", withContext(a), &res, d); err != nil {
		if errors.Is(err, rex.ErrTimeout) {
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

type doOpts struct {
	a                  rex.Args
	args, eval, action string
	json               bool
}

func cliDo(o *doOpts, f *call) error {
	a, argsJSON, eval, jsonOut := &o.a, o.args, o.eval, o.json
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
	if err := do("client.do", a, &res); err != nil {
		return err
	}
	if jsonOut && len(res) > 0 {
		var v any
		_ = json.Unmarshal(res, &v) // the server's JSON; null if it were not
		printJSON(v)
	}
	return nil
}

func cliActions(o *bool, _ *call) error {
	jsonOut := *o
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

func cliKeymap(o *bool, _ *call) error {
	jsonOut := *o
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
	return keymapErrors(km.problems)
}

// keymapErrors fails when a problem of a keymap is an error, for rex
// keymap and rex config check alike.
func keymapErrors(problems []string) error {
	for _, p := range problems {
		if strings.HasPrefix(p, "error") {
			return errors.New("the keymap has errors")
		}
	}
	return nil
}

// configCheck is what rex config check finds in a keymap.
type configCheck struct {
	Path     string   `json:"path"`
	Exists   bool     `json:"exists"`
	Preset   string   `json:"preset"`
	Bindings int      `json:"bindings"`
	FromFile int      `json:"from_file"`
	Problems []string `json:"problems"`
}

func cliConfigCheck(o *bool, f *call) error {
	path := keymapPath()
	if len(f.args) > 0 {
		path = f.args[0]
	}
	loadSettingsQuiet()
	km := loadKeymap(path)
	r := configCheck{Path: path, Exists: km.exists, Preset: presetOf(path), Bindings: len(km.bindings), Problems: km.problems}
	if r.Problems == nil {
		r.Problems = []string{}
	}
	for _, b := range km.bindings {
		if b.source != "default" {
			r.FromFile++
		}
	}
	output(*o, r, func() {
		exists := "no"
		if r.Exists {
			exists = "yes"
		}
		fmt.Printf("path: %s\nexists: %s\npreset: %s\nbindings: %d (%d from the file)\n", path, exists, r.Preset, r.Bindings, r.FromFile)
		for _, p := range r.Problems {
			fmt.Println(p)
		}
	})
	return keymapErrors(r.Problems)
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

// cliConfigReload has the app read its keymap again, and checks it. With
// no app connected it fails, as rex do does.
func cliConfigReload(o *bool, _ *call) error {
	var res json.RawMessage
	if err := do("client.do", &rex.Args{Action: "client.config.reload", Client: opts.client}, &res); err != nil {
		return err
	}
	return cliConfigCheck(o, &call{})
}

func cliServerStatus(o *bool, _ *call) error {
	jsonOut := *o
	c, err := rex.DialFor(opts.timeout)
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

type stopOpts struct{ discard, ifIdle bool }

func cliServerStop(o *stopOpts, _ *call) error {
	c, err := rex.DialFor(opts.timeout)
	if err != nil {
		if o.discard {
			return rex.DiscardState()
		}
		return &cliError{exitNoServer, errors.New("the server is not running")}
	}
	defer c.Close()
	switch {
	case o.ifIdle:
		err = c.ShutdownIdle(o.discard)
		if re, ok := errors.AsType[*rex.Error](err); ok && strings.HasPrefix(re.Msg, "unknown op") {
			return &cliError{exitNoServer, errors.New("the server is too old for --if-idle: it goes on")}
		}
	case o.discard:
		err = c.ShutdownForget()
	default:
		err = c.Shutdown()
	}
	if err != nil {
		return err
	}
	// The server answers, then saves and ends: rex returns once it has
	// ended, so that "rex server stop; rex server start" starts a new one.
	c.Close()
	for deadline := time.Now().Add(stopWait()); serverRuns(); {
		if time.Now().After(deadline) {
			return errors.New("the server did not stop in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if o.discard {
		return rex.DiscardState() // the saved sessions go once it has stopped
	}
	return nil
}

// stopWait is how long server stop waits for the server to end: the
// timeout, and 10 seconds when the timeout is off.
func stopWait() time.Duration {
	if opts.timeout > 0 {
		return opts.timeout
	}
	return 10 * time.Second
}

func cliServerStart(args []string) error {
	c, err := rex.ConnectFor(opts.timeout)
	if err != nil {
		return &cliError{exitNoServer, err}
	}
	c.Close()
	return nil
}

func cliServerRun(args []string) error { return rex.Serve() }

type logsOpts struct {
	follow, all bool
	lines       int
}

// logLines is how many of its newest lines rex server logs prints, as
// Rex's.
const logLines = 200

// cliServerLogs prints the newest lines of server.log, or all of it;
// with --follow, it goes on printing what the server adds, until the
// server stops.
func cliServerLogs(o *logsOpts, _ *call) error {
	if o.all && o.lines > 0 {
		return usagef("--all or -n, not both")
	}
	path := filepath.Join(rex.Dir(), "server.log")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	at := int64(len(b))
	if !o.all {
		n := o.lines
		if n == 0 {
			n = logLines
		}
		b = lastLines(b, n)
	}
	os.Stdout.Write(b)
	if !o.follow {
		return nil
	}
	for {
		time.Sleep(200 * time.Millisecond)
		if b, at, err = readFrom(path, at); err != nil {
			return err
		}
		os.Stdout.Write(b)
		if len(b) == 0 && !serverRuns() {
			return nil
		}
	}
}

// lastLines are the last n lines of b.
func lastLines(b []byte, n int) []byte {
	end := len(b)
	if end > 0 && b[end-1] == '\n' {
		end--
	}
	for i := end - 1; i >= 0; i-- {
		if b[i] == '\n' {
			if n--; n == 0 {
				return b[i+1:]
			}
		}
	}
	return b
}

// readFrom reads a file from offset at, and returns where it ended; from
// the start again when the file is shorter, as after it was replaced.
func readFrom(path string, at int64) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, at, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, at, err
	}
	if fi.Size() < at {
		at = 0
	}
	b, err := io.ReadAll(io.NewSectionReader(f, at, fi.Size()-at))
	return b, at + int64(len(b)), err
}

// serverRuns tells whether a server answers on the socket.
func serverRuns() bool {
	c, err := rex.DialFor(opts.timeout)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// cliStatus reports a program's status for a block, as OSC 7501 would
// from its terminal, or, with no state, prints the block's records.
type statusOpts struct {
	a                                   rex.Args
	app, msg, kind, progress, title, id string
	json                                bool
}

func cliStatus(o *statusOpts, f *call) error {
	a, jsonOut := &o.a, o.json
	if len(f.args) == 0 {
		var res struct {
			Block   string             `json:"block_id"`
			Records []rex.StatusRecord `json:"records"`
		}
		if err := do("status.list", a, &res); err != nil {
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
	a.Data = statusReport(f.args[0], o.id, o.kind, o.progress, o.app, o.title, o.msg)
	_, err := resolveIDs("status.report", a)
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
