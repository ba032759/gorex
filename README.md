# GoRex

A replica of [Superlogical's Rex](https://www.superlogical.com/updates/public-testing-beginning)
terminal, for Linux only, written in Go with [MyGo](https://mygo.egoist.dev)'s native UI and its
terminal plugin (Ghostty's libghostty-vt). No webview, no cgo: one ~15 MB app.

![GoRex](docs/screenshot.png)

## What it does

- **Sessions, tabs and split panes**, as Rex has them: a *session* groups
  related work, its *windows* are the tabs, and each *block* is a terminal.
  The session chip in the title bar (or Ctrl+Shift+S) changes
  session; its dot tells of a pane needing attention in another one. Every
  pane is a card over the window's gradient, with its program's icon, name
  and directory in its header, and buttons to split right, split down, zoom
  and close. Drag the gaps between panes to resize them (double-click resets
  a split to half).
- **The server owns everything.** Sessions, their tabs, layouts, labels,
  focus and zoom live in a session server, a background process of the same
  binary (`GoRex -server`); the window mirrors it. Quit GoRex and
  everything keeps running; open it again and it is all there, programs
  still running. The server keeps each block's screen in a headless
  libghostty-vt emulator and sends a snapshot on attach, so full-screen
  programs (lazygit, vim, htop…) come back exactly. When the server itself
  ends (a reboot), it starts every block's shell again in the directory it
  was in, with the same IDs and labels.
- **The `rex` command**, Rex's CLI: every terminal has it on its `PATH`
  and knows where it runs (`REX_SESSION`, `REX_BLOCK`), so `rex split --
  htop` splits the terminal you type it in. It works with or without the
  window open. See [The CLI](#the-cli).
- **Agents.** `rex skill setup` (or *Install Rex for Agents* in the
  palette) teaches Claude Code, Codex, OpenCode and Pi to open panes, run
  commands beside them, wait for them and read terminals' screens.
- **Program activity.** The server watches each terminal's foreground
  process (`tcgetpgrp`, `/proc`): headers and tabs name the program (`Node`, `Git
  Changes` for lazygit, `Codex`, `SSH host`…) and its working directory, a
  green dot shows a program printing, and an orange dot marks a pane whose
  program finished or rang the bell out of sight. When the window is in the
  background, a long command finishing posts a notification.
- **Program status** ([OSC 7501](https://www.superlogical.com/rex/docs/build/program-status)):
  a program tells its terminal what it does — working (with progress),
  blocked on you for a permission, a question or a login, done, failed —
  and the pane's header, its tab, and the session chip show it; a pane out
  of sight that becomes blocked, done or failed posts a notification. `rex
  status` reports it for programs that cannot print it, and `rex skill
  setup` hooks Claude Code in, so its tab says *needs permission* rather
  than just printing.
- **Tab icons** stack the tiles of a tab's programs, the focused pane's in
  front, as Rex does. Tabs reorder by dragging, rename by double-click, and
  have a context menu.
- **Host chip** with the machine's name and model; its popover shows the
  chip, memory, OS and the session server.
- **Command palette** (Ctrl+Shift+P, or the ⌘ button): every command,
  session and pane to jump to, fuzzy-matched; it asks for names too
  (Rename Pane…, Rename Session…).
- **A keymap file** with Rex's key syntax and action names, key sequences
  (`ctrl+b>c`) included. See [Keys](#keys).
- Light and dark appearances (View ▸ Appearance), text size (Ctrl+= Ctrl+- Ctrl+0),
  JetBrains Mono embedded.

## Keys

| Keys                                           |                                  |
| ---------------------------------------------- | -------------------------------- |
| Ctrl+Shift+T                                   | New tab                          |
| Ctrl+Shift+N                                   | New session                      |
| Ctrl+Shift+S                                   | Change session                   |
| Ctrl+Shift+D / Ctrl+Shift+E                    | Split right / down               |
| Ctrl+Shift+W                                   | Close pane                       |
| Ctrl+Shift+Enter                               | Zoom the pane                    |
| Alt+arrows                                     | Focus the pane in that direction |
| Alt+Shift+arrows                               | Move the nearest divider         |
| Ctrl+Shift+1…9, Ctrl+PageUp/PageDown, Ctrl+Tab | Switch tabs                      |
| Ctrl+Shift+PageUp/PageDown                     | Move the tab                     |
| Ctrl+Shift+R                                   | Rename tab                       |
| Ctrl+Shift+P                                   | Command palette                  |
| Ctrl+Shift+K                                   | Clear                            |
| Ctrl+= Ctrl+- Ctrl+0                           | Text size                        |

Control and a letter are the terminal's (Ctrl+C, Ctrl+D, Ctrl+W…), so
GoRex's keys take Shift too, as copying and pasting do (Ctrl+Shift+C,
Ctrl+Shift+V), and none take Control and Alt, which type
characters (AltGr) on many keyboards. With `"macosKeybindings": true` in
`settings.json`, for a keyboard remapped to work as a Mac's, GoRex takes
the macOS keys with Control or Super for ⌘ instead (the `macos` preset).

### keymap.json

`keymap.json`, beside `settings.json` (*Open Keymap File* in the palette
writes an example), changes the keys:

```json
{
  "preset": "linux",
  "bindings": [
    {"key": "ctrl+b>c", "action": "client.tab.new"},
    {"key": "ctrl+b>%", "action": "pane.split", "args": {"direction": "right"}},
    {"key": "ctrl+b>z", "action": "pane.zoom"},
    {"key": "ctrl+shift+k", "action": "none"}
  ]
}
```

Keys are written as Rex writes them: modifiers (`ctrl`, `alt`, `shift`,
`super` or `cmd`) and a key joined by `+`, without shift applied (plus is
`ctrl+shift+=`), and sequences joined by `>`; `"none"` unbinds a key, which
then goes to the terminal. Actions have Rex's names (`pane.split.right`,
`client.tab.goto`, `session.switch`…): `rex actions` lists them, `rex
keymap` shows the bindings in effect, `rex config check` checks the file
and `rex config reload` (or *Reload Keymap*) applies it.

## The CLI

`rex` follows [Rex's CLI](https://www.superlogical.com/rex/docs/automate/cli):
the same commands, flags, targets and exit statuses (0 ok, 1 failed, 2
usage, 3 target not found or ambiguous, 4 server unreachable; `wait` and
`--wait` exit with the program's status). Targets (`-s`, `-w`, `-b`) are a
full ID, a label, a window's position, or the last 4+ characters of an ID.

```sh
rex ls                                    # sessions
rex new api --cwd ~/src/api -- nvim       # a session
rex split --split=below --ratio 30 --label logs -- tail -f app.log
rex run --wait -- go test ./...           # a new tab; exits with the tests' status
B=$(rex split --json --focus=false -- make | jq -r '.block_ids[0]')
rex wait --for 10m "$B"
rex send -b logs 'make test' && rex send-key -b logs enter
rex capture -b logs --trim
rex focus left; rex zoom --on; rex move logs right editor; rex swap a1b2 c3d4
rex session|window|block ls|inspect|rename|move|close…
rex events -s work --json | jq -c .       # what happens, as it happens
rex status working --msg "Deploying"      # a program's status (OSC 7501)
rex do session.select session=work        # an action of the window
rex skill setup                           # Claude Code, Codex, OpenCode, Pi
```

`rex help` lists everything, `rex help COMMAND` or `rex COMMAND --help` one command. Run as `rex`
(the server links `bin/rex` in its directory to itself and puts it first
in its terminals' `PATH`), or as `GoRex COMMAND`. Not in GoRex: Lua
(`init.lua`, `rex do script.lua`), remote hosts (`-S`, `rex hosts`,
Tailscale), and `capture --format html`.

## Develop and build

GoRex needs [Go](https://go.dev/dl/) 1.27+ and MyGo 0.3.0, whose CLI
`go.mod` pins as a tool:

```sh
go tool mygo dev     # GoRex Dev, rebuilt and restarted as the code changes
go test ./...        # the server, and the view without a window
go tool mygo build   # the app, in build/
go run ./tools/mkicon  # redraw resources/icon.png
```

`go get -tool github.com/egoist/mygo/cmd/mygo@latest` updates MyGo and its
CLI together.

The app keeps its state in its data directory: the server's socket and
log, `state.json`, `settings.json` and `keymap.json`, in
`$XDG_CONFIG_HOME/gorex`, else `~/.config/gorex`, for the
built app and `GoRex Dev` for `mygo dev`'s, so that developing never
touches the sessions of the app you use (`GOREX_DIR` names another
directory; a terminal of GoRex sets it, so that its `rex` talks to the
server that runs it). The sessions outlive the app, but not a rebuild: a
development build replaces a server that an older build started, ending
its programs, which then start again as shells in the same directories.
An app of a new version asks before it restarts a server of an older one;
the first version's `layout.json` becomes a session.

## Layout

|                                                              |                                                                                                                                                                                                                   |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `main.go`                                                    | the app, its window, `-server`, and running as `rex`                                                                                                                                                              |
| `state.go`                                                   | the mirror of the server's model: tabs, the tree of splits, panes; the user's actions                                                                                                                             |
| `view.go`, `tabs.go`, `sessions.go`, `host.go`, `palette.go` | the interface: title bar, tabs, session chip, panes, host popover, palette                                                                                                                                        |
| `commands.go`, `keymap.go`                                   | the actions, by Rex's names; menus; the keymap and its presets                                                                                                                                                    |
| `cli.go`, `help.go`, `skill.go`                              | the `rex` command; `rex skill`                                                                                                                                                                                    |
| `programs.go`                                                | how programs show: names, glyphs, tile colors                                                                                                                                                                     |
| `style.go`, `settings.go`                                    | colors, fonts, terminal themes; appearance and text size                                                                                                                                                          |
| `internal/rex`                                               | the session server and its client: the model of sessions, windows and layouts (`model.go`, `actions.go`), blocks' PTYs and headless screens (`block.go`), foreground processes, keys, events, the socket protocol |

Not replicated: Rex's connections to servers on other machines (GoRex's
server listens on a local socket only), its Lua configuration and
scripts, and its floating panes.

Icons: [Lucide](https://lucide.dev) (ISC) and [Simple Icons](https://simpleicons.org)
(CC0). Font: [JetBrains Mono](https://www.jetbrains.com/lp/mono/) (OFL).
