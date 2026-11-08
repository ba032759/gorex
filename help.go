package main

import (
	"fmt"
	"strings"
)

// help is rex's help, by topic: "" for the overview, a command or group.
var help = map[string]string{
	"": `rex drives GoRex: sessions of tabs (windows) of terminals (blocks), kept
by a server that outlives the app. Inside a GoRex terminal it knows where it
runs, from REX_SESSION and REX_BLOCK.

Usage: rex [-C CLIENT] [--timeout 10s] [--autostart=false] COMMAND [ARGS]

Common commands
  ls                          list sessions
  new [LABEL] [-- CMD...]     create a session
  run [flags] -- CMD...       run a command in a new tab, or beside a block (--split)
  split [flags] [-- CMD...]   split a block (yours, by default)
  send [TEXT]                 type text into a terminal (stdin without TEXT)
  send-key KEY...             press keys: enter, ctrl+c, shift+tab, up…
  capture                     print a terminal's screen and scrollback
  kill SESSION...             end sessions

Layout
  focus TARGET|DIRECTION      focus a block or window, or left/right/up/down/next/prev
  zoom [BLOCK]                zoom a block to fill its window (--on, --off)
  move [BLOCK] DIR ANCHOR     move a block next to another
  swap BLOCK OTHER            swap two blocks
  resize [BLOCK] DIR          move a block's divider (--amount PCT)
  close [BLOCK...]            close blocks

Resources
  session new|ls|inspect|rename|move|kill
  window  new|ls|inspect|focus|move|rename|close
  block   ls|inspect|focus|send|send-key|capture|zoom|move|swap|rename|close
  client  ls|inspect
  list                        the whole tree of sessions, windows and blocks

Automation
  events [-s SESSION] [--all] [--json]   stream what happens
  wait TARGET [--for 10m]     wait for a block's program (exits with its status), a window or a session
  status [STATE] [--msg ...]  report a program's status (OSC 7501), or show a block's
  do ACTION [KEY=VALUE...]    perform an action of the app, as its keys do
  actions                     list the app's actions
  keymap                      list the key bindings in effect
  config check|reload         check, or have the app reload, keymap.json
  skill setup|status|remove|show   set up agents (Claude Code, Codex…) to use rex

Server
  server status|stop|start|logs
  version, completion bash|zsh|fish

Targets (-s SESSION, -w WINDOW, -b BLOCK): a full ID, a label, a window's
position from 1, or the last 4+ characters of an ID.

Exit status: 0 ok, 1 failed, 2 usage, 3 target not found or ambiguous,
4 server unreachable; wait and --wait exit with the program's status.

Run "rex help COMMAND" for one command.
`,
	"split": `rex split [flags] [-- COMMAND...]
rex run [flags] -- COMMAND...

split splits a block (yours by default) and places a terminal beside it;
run places one in a new tab, or beside a block with --split.

  --split DIRECTION   right (split's default), left, above or below
  --ratio PCT         the new block's share of the split (default 50)
  -b, --block BLOCK   the block to split (default: yours, else the focused)
  -s, --session S     -w, --window W   where to act
  -c, --cwd DIR       its working directory (default: the anchor's)
  --label TEXT        a label to target it by
  --focus=false       leave the focus where it is
  --keep-open         keep the terminal after its command exits
  --shell SHELL       run COMMAND through SHELL's login environment (none: directly)
  --interactive       also read the shell's interactive startup files
  --wait              wait for COMMAND, exiting with its status
  --json              print the result as JSON (block_ids, split_ids…)
`,
	"capture": `rex capture [-b BLOCK] [-s S] [-w W] [--trim] [--format text|vt]

Prints a terminal's screen and scrollback. --trim drops trailing blank
lines; --format vt keeps the escape sequences. Lines a program's long
output wrapped come joined.
`,
	"send-key": `rex send-key [-b BLOCK] KEY...

Keys as Rex writes them: modifiers (ctrl, alt, shift) and a key joined by
"+": enter, tab, escape, backspace, space, delete, up, down, left, right,
home, end, pageup, pagedown, f1-f12, or a character. [KeyA] names a key by
its place on a US keyboard.
`,
	"skill": `rex skill setup [--agent NAME]... [--project] [--dry-run] [--force] [--session-naming] [--json]
rex skill status [--project] [--json]
rex skill remove [--agent NAME]... [--project] [--dry-run] [--json]
rex skill show

Installs a skill that teaches agents rex (Claude Code: ~/.claude/skills/rex;
Codex, OpenCode, Pi: ~/.agents/skills/rex) and, in Claude Code, a
SessionStart hook in ~/.claude/settings.json that tells the agent which
session and terminal it runs in. The rest of the settings file stays as it
was. Run setup again to update; remove takes out only what rex installed.
`,
	"keymap": `rex keymap [--json]

Lists the key bindings in effect: the preset's and keymap.json's, beside
settings.json in GoRex's data directory. keymap.json:

  {
    "preset": "linux",
    "bindings": [
      {"key": "ctrl+b>c", "action": "client.tab.new"},
      {"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
      {"key": "ctrl+shift+k", "action": "none"}
    ]
  }

Presets: linux (Ctrl+Shift, the default), macos (the Mac's shortcuts with
Ctrl or Super for Cmd), none. Keys: modifiers (ctrl, alt, shift, super or
cmd) and a key joined by "+", sequences joined by ">". "none" unbinds a
key. "rex actions" lists the actions; "rex config reload" applies changes.
`,
	"status": `rex status [-b BLOCK] [--json]
rex status STATE [-b BLOCK] [--app APP] [--msg TEXT] [--kind KIND] [--progress PCT] [--title TEXT] [--id ID]

Program status (the OSC 7501 protocol): what a program in a terminal is
doing, which tabs, headers and notifications show. A program can print it
to its terminal:

  printf '\033]7501;state=working:app=build:msg=%s\033\\' "$(printf 'Building' | base64)"

or report it with rex status, as a hook that has no terminal does. STATE
is idle, working, blocked (with --kind permission, question or auth),
done, error, or clear. working and blocked end with the program, or when
the shell shows its prompt; done and error stay until the user looks at
them. Without STATE it prints the block's records. rex skill setup hooks
Claude Code in, so that its tab tells when it works, waits for permission,
or finished.
`,
	"do": `rex do ACTION [KEY=VALUE...] [--args JSON] [-C CLIENT] [--json]

Asks the GoRex app to perform one of its actions, as its keys do:
  rex do pane.split direction=below
  rex do session.select session=work
  rex do client.tab.goto index=2
"rex actions" lists them. GoRex has no Lua: scripts are shell scripts.
`,
	"events": `rex events [-s SESSION] [--all] [--json]

Prints events as they happen: sessions created and destroyed, and with -s,
everything in one session, its terminals' events too (bell, child_exited,
title_changed, pwd_changed, process_changed). --all prints every event.
`,
	"wait": `rex wait TARGET [-s S] [-w W] [--for DURATION] [--json]

Waits for a block's program to exit, and exits with its status; or for a
window or session to close. --for gives up after DURATION, with status 1.
`,
}

func printHelp(topic string) {
	topic = strings.TrimSpace(topic)
	switch topic {
	case "run":
		topic = "split"
	case "block capture":
		topic = "capture"
	case "block send-key":
		topic = "send-key"
	}
	if h, ok := help[topic]; ok {
		fmt.Print(h)
		return
	}
	fmt.Print(help[""])
}
