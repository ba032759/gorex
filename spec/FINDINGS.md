# Findings

What specifying `rex` after the fact found. Competing behaviour first:
two parts of GoRex doing one thing differently. Nothing here is fixed
yet; fixing a finding is a change of its own.

## Finding 1: --ratio takes different ranges in split and move

- Kind: competing behaviour
- Items: F-135, F-145
- Code: `cli.go:1008`, `cli.go:1200`

`rex split` and `rex run` take a `--ratio` above 0 and below 100, so
`--ratio 0.5` gives the new block half a percent. Their error message
says "between 1 and 99". `rex move` takes 1 to 99 only, with the same
message. One flag, one meaning, two ranges.

## Finding 2: rex focus DIRECTION starts from another block than every other command

- Kind: competing behaviour
- Items: F-112, F-142
- Code: `internal/rex/actions.go:771`, `internal/rex/actions.go:316`

Every command without a target acts on the context block (F-112):
`rex split` splits the terminal it runs in. `rex focus left` alone
starts from the focused block of the context window, not from the
context block. The comment says this is meant. Rex's documentation says
"here" is the agent's own block, and "the focused pane" the focused one,
so both readings exist. The owner decides which a direction starts from.

## Finding 3: a new block's directory depends on the command that makes it

- Kind: competing behaviour
- Items: F-131, F-134, F-135, F-150
- Code: `cli.go:548`, `cli.go:768`, `cli.go:1002`, `internal/rex/actions.go:654`

Without `--cwd`, `rex new` starts in the directory `rex` runs in, with or
without a command. `rex run` does the same, only with a command. `rex
split` starts in the anchor's directory. `rex window new` starts in
`rex`'s directory with a command and in the focused block's directory
without one. A script that opens a session and a window in a row gets
two different directories.

## Finding 4: --timeout does not limit most requests

- Kind: competing behaviour
- Items: F-101, F-168
- Code: `internal/rex/client.go:144`, `cli.go:449`, `cli.go:668`, `cli.go:1307`, `cli.go:409`, `cli.go:1631`

Requests sent with `do` take `--timeout` (default 10 s). Listings and
target lookups (`model`, `list`, `clients`, `*.resolve`) use a fixed
15 s. Connecting with autostart has no limit at all; only
`--autostart=false` dials with `--timeout`. `rex wait` tells its own
timeout from other errors by the words "timed out" in the message.

## Finding 5: "no app" fails in rex do and succeeds in rex config reload

- Kind: competing behaviour
- Items: F-175, F-180
- Code: `cli.go:1814`, `internal/rex/server.go:370`, `cli.go:58`

With no app connected, `rex do` exits with 1 and `rex config reload`
prints a note and exits with 0. Both ask the app for an action. The
server's code `no_client` has no exit status of its own.

## Finding 6: the server going away gives exit status 4 or 1

- Kind: competing behaviour
- Items: F-104, F-168, F-169
- Code: `cli.go:1555`, `internal/rex/actions.go:1116`, `internal/rex/client.go:116`

When the server ends, `rex events` exits with 4 ("server unreachable").
`rex wait`, waiting on the same server, exits with 1.

## Finding 7: two ways to match a client

- Kind: competing behaviour
- Items: F-113, F-156, F-175
- Code: `cli.go:1525`, `internal/rex/actions.go:1143`

`rex do -C` matches a client by the server's rules (F-113): full ID,
name in any case, position, end of ID of at least 4 characters. `rex
client inspect` matches in `rex` itself: full ID, an end of ID of any
length, or the exact name. `rex client inspect 1` matches every client
whose ID ends in `1`, and prints the first.

## Finding 8: one fact, two JSON names

- Kind: competing behaviour
- Items: F-152, F-153, F-166, T-107, T-108
- Code: `internal/rex/proto.go:165`, `internal/rex/proto.go:170`, `internal/rex/model.go:76`

`rex block ls --json` prints `exitCode`, `lastOutput` and `lastInput`.
`rex list --json`, `rex wait --json` and the model print `exit_code`.
Every other key is in snake case, as Rex's are.

## Finding 9: on Windows, REX_BIN_DIR names a directory without rex

- Kind: competing behaviour
- Items: F-118, F-119, F-186
- Code: `internal/rex/server.go:219`, `internal/rex/block.go:239`, `skill.go:360`

Every block gets `REX_BIN_DIR` and that directory first in `PATH`. The
server makes the `rex` link there on Linux and macOS only. On Windows the
directory is empty, so Claude Code's hooks (`"$REX_BIN_DIR/rex"`) do
nothing, and `rex` is not on a block's `PATH` unless installed elsewhere.

## Finding 10: commands with several targets stop half done

- Kind: difference from Rex
- Items: F-113, F-140, F-141, F-151
- Code: `cli.go:742`, `cli.go:962`, `cli.go:1279`

Rex's documentation: an ambiguous target changes nothing. `rex kill a b
c`, `rex close a b c` and `rex window close a b c` act on each target in
turn. When `b` is ambiguous, `a` is already gone.

## Finding 11: rex sets REX_SERVER and does not read it

- Kind: difference from Rex
- Items: F-106, F-118, F-120
- Code: `internal/rex/block.go:253`, `cli.go:405`, `internal/rex/proto.go:229`

Blocks get `REX_SERVER=unix:SOCKET`, as Rex's do. `rex` finds its server
by `GOREX_DIR` instead, and refuses `-S`, whose default in Rex is
`REX_SERVER`. A block whose `GOREX_DIR` is removed reaches another
server than the one `REX_SERVER` names.

## Finding 12: GOREX_SESSION holds a block's ID

- Kind: inconsistent terms
- Items: F-109, F-118
- Code: `internal/rex/block.go:258`, `cli.go:437`

`GOREX_SESSION` is the block's ID, from the first version, where a
"session" was one terminal. `rex` still reads it when `REX_BLOCK` is
empty.

## Finding 13: block, pane and terminal; window and tab

- Kind: inconsistent terms
- Items: glossary (block, window, close), F-140, F-141
- Code: `help.go:10`, `skill.go:193`, `commands.go` (action names), `README.md`

Rex's CLI and the model say session, window and block. The app and the
README say tab and pane, and the action names are Rex's own
(`pane.split`, `client.tab.new`). `rex help` says "tabs (windows) of
terminals (blocks)"; the skill says "a block is one terminal (a pane)".
The spec takes Rex's words: window (avoid tab), block (avoid pane).
`rex kill` ends sessions and `rex close` closes blocks; the spec says
close, and avoids kill.

## Finding 14: the data directory on Linux

- Kind: inconsistent terms
- Items: F-106, T-120
- Code: `README.md:167`, `AGENTS.md:35`, `internal/rex/proto.go:237`, `main.go:69`

The README says `~/.config/gorex` on Linux. `AGENTS.md`, `wide.go` and
the code say `GoRex`: `rex.Dir()` joins the user's configuration
directory and `GoRex`, and the app sets `GOREX_DIR` from MyGo's data
directory of the app.

## Finding 15: help that says less, or other, than the code

- Kind: other
- Items: F-103, F-169, F-178, F-186
- Code: `help.go:152`, `help.go:102`, `help.go:120`, `keymap.go:269`, `cli.go:302`

- `rex help events`: without `-s`, only "sessions created and
  destroyed". The code also prints every session's window and block
  events.
- `rex help skill`: "a SessionStart hook". Setup adds seven hooks.
- `rex help keymap`: presets `linux`, `macos`, `none`. On macOS the
  default is `darwin`.
- `rex session` without a subcommand prints `help["session"]`, which
  does not exist, so the overview prints.

## Finding 16: global options work only by the name rex

- Kind: accidental behaviour
- Items: F-100, F-101
- Code: `main.go:38`, `cli.go:99`

`GoRex ls` runs `rex ls`. `GoRex --timeout 5s ls` starts the app, as
`--timeout` is not a command. `rex --timeout 5s ls` works.

## Finding 17: flags taken and ignored

- Kind: accidental behaviour
- Items: F-101, F-161, F-164, F-176
- Code: `cli.go:128`, `cli.go:130`, `cli.go:1091`, `cli.go:1119`, `cli.go:1653`

`--color`, `-v`, `capture --unwrap`, `send-key --action repeat`, and `do
--origin`, `--for`, `--all`, `--fail-fast`, `-a` are taken so that Rex's
command lines run. They change nothing. Rex's scripts then get
different results with exit status 0.

## Finding 18: Rex's flags that GoRex refuses with exit status 2

- Kind: difference from Rex
- Items: F-156, F-179, F-180, F-182, F-183
- Code: `cli.go:1767`, `cli.go:1856`, `cli.go:1487`, `cli.go:1881`

`rex config check --json`, `rex config reload --json`, `rex server stop
--discard` and `--if-idle`, `rex server logs --follow` and `-n`, `rex
client ls -q` and `--short` exit with 2. GoRex's `server stop --forget`
is Rex's `--discard` under another name.

## Finding 19: what Rex has and rex does not

- Kind: difference from Rex
- Items: F-101, F-164, F-165, F-176, F-177, F-124, F-185, F-170
- Code: `cli.go:112`, `cli.go:1123`, `internal/rex/block.go:23`, `cli.go:1712`, `skill.go:61`, `internal/rex/actions.go:1243`

- `-S`, `rex hosts`, `home`, `whoami`, `login-server`, `server
  tailscale`: refused or unknown (D-5).
- `rex api`, `rex block stream`, `rex block call`: unknown (D-7).
- Lua in `rex do`, `init.lua` (D-4).
- `capture --format html`: refused. Scrollback is 8 MiB, not 64 MiB.
  `capture` always joins wrapped lines; Rex does with `--unwrap`.
- `rex actions` lists the app's actions from `rex` itself; Rex asks the
  client named by `-C`.
- `completion powershell` and `nu`: refused.
- `rex skill setup` installs no hook for Codex; Rex writes
  `~/.codex/hooks.json`.
- `process_changed` has `foreground` and `args`, not Rex's `generation`
  and `ancestors`; `pwd_changed` and `title_changed` have no `owner`.
- Exit status 130 for an interrupted command is not specified: Go's
  default handling of `SIGINT` decides it.

## Finding 20: rex status is GoRex's own

- Kind: difference from Rex
- Items: F-172, F-173
- Code: `cli.go:1910`

Rex's CLI reference has no `rex status`; Rex's programs print OSC 7501
themselves. GoRex adds it for hooks that have no terminal.

## Finding 21: what the tests cannot show

- Kind: untestable
- Items: F-100, F-107, F-119, F-121, F-184
- Code: `main.go:78`, `internal/rex/server.go:970`, `internal/rex/server.go:218`, `cli.go:161`, `internal/rex/server.go:111`

Running by the name `rex`, starting a server (it runs `os.Executable`,
the test binary), the `bin/rex` link, the wide event of a command, and a
server exiting after 10 idle minutes have no test. F-121 says `verify:
event`; the others need a test that builds the executable.

## Finding 22: the matrix shows nothing of a retro spec

- Kind: spec rules
- Items: all
- Code: `tools/spec/main.go:599`, `tools/spec/main.go:604`

`spec matrix` lists only accepted business and functional items. Every
item of this pass is `proposed`, so the matrix is empty until the owner
accepts an area. Coverage of proposed items is visible only through
`spec show` or `spec query`.

## Finding 23: tables escape the budgets

- Kind: spec rules
- Items: F-118, F-163, T-104, T-105, T-110
- Code: `tools/spec/main.go:854`

`spec check` leaves tables out of the word count and the sentence check.
The tables of exhaustive cases that `spec/README.md` asks for are free,
so F-118 or T-105 hold far more than 150 words of content. The budget
then limits prose, not size.

## Finding 24: the sentence limit is 25, not 20

- Kind: spec rules
- Items: all functional items
- Code: `tools/spec/main.go:841`

`spec/README.md` sets 20 words for a requirement and 25 for a
description. `spec check` warns at 25 for both. Some requirements here
have 21 to 25 words.

## Finding 25: a term and the product's name differ only in case

- Kind: spec rules
- Items: glossary
- Code: `tools/spec/main.go:916`

The glossary compares terms without case, so "Rex" and "rex" are one
term. The glossary names the command "rex command".

## Finding 26: avoided words have one meaning in the spec, more in the code

- Kind: spec rules
- Items: glossary (window, block, close)
- Code: `tools/spec/main.go:949`

`avoid:` is checked by word, not by meaning. With `tab` avoided for
window, the spec cannot name the app's tab bar, or Rex's `Ctrl+Tab`
outside backticks. The GUI area needs `tab` for what the app draws.

## Finding 27: no source for the repository's documents

- Kind: spec rules
- Items: all
- Code: `tools/spec/main.go:828`

`source:` takes `owner`, `rex-docs`, `code` and `tests`. The README and
the git history are neither code nor tests; this pass marks them `code`.
A `repo-docs` source would show where an item rests on prose a person
wrote.
