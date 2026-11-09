# Findings

What specifying `rex` after the fact found. Competing behaviour first:
two parts of GoRex doing one thing differently. Nothing here is fixed
yet; fixing a finding is a change of its own.

## Finding 1: --ratio takes different ranges in split and move

- Kind: competing behaviour
- Items: F-135, F-145
- Code: `cli.go:1109`, `cli.go:204`

`rex split` and `rex run` take a `--ratio` above 0 and below 100, so
`--ratio 0.5` gives the new block half a percent. Their error message
says "between 1 and 99". `rex move` takes 1 to 99 only, with the same
message. One flag, one meaning, two ranges.

Changed by D-13 (proposed): each range is now one declaration, which
the help and the reference show ("above 0 and below 100", "1 to 99").
Making them one is a change of one line; the choice of range is the
owner's.

- Status: resolved (2026-11-09): every `--ratio` takes 1 to 99, as Rex's reference says; one message from the range (D-14).

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

- Status: decided: a direction starts from the context block, REX_BLOCK when it is live, else the focused block (F-112).

## Finding 3: a new block's directory depends on the command that makes it

- Kind: competing behaviour
- Items: F-132
- Code: `cli.go:548`, `cli.go:768`, `cli.go:1002`, `internal/rex/actions.go:654`

Without `--cwd`, `rex new` starts in the directory `rex` runs in, with or
without a command. `rex run` does the same, only with a command. `rex
split` starts in the anchor's directory, but `rex run --split`, which
makes the same split, starts in `rex`'s (`cli.go:1002`). `rex window new`
starts in `rex`'s directory with a command and in the focused block's
directory without one. A script that opens a session and a window in a
row gets two different directories.

- Status: decided: beside a block (`split`, `run --split`), its directory; otherwise (`new`, `run`, `window new`), the directory `rex` runs in.

## Finding 4: --timeout does not limit most requests

- Kind: competing behaviour
- Items: F-101, F-168
- Code: `internal/rex/client.go:144`, `cli.go:449`, `cli.go:668`, `cli.go:1307`, `cli.go:409`, `cli.go:1631`

Requests sent with `do` take `--timeout` (default 10 s). Listings and
target lookups (`model`, `list`, `clients`, `*.resolve`) use a fixed
15 s. Connecting with autostart has no limit at all; only
`--autostart=false` dials with `--timeout`. `rex wait` tells its own
timeout from other errors by the words "timed out" in the message.

- Status: decided: `--timeout` limits every connection and every request that does not stream, as Rex's reference says.

## Finding 5: "no app" fails in rex do and succeeds in rex config reload

- Kind: competing behaviour
- Items: F-175, F-180
- Code: `cli.go:1814`, `internal/rex/server.go:370`, `cli.go:58`

With no app connected, `rex do` exits with 1 and `rex config reload`
prints a note and exits with 0. Both ask the app for an action. The
server's code `no_client` has no exit status of its own.

- Status: resolved (2026-11-09): an app action with no app connected exits with 1, for `rex do` and `rex config reload` (F-175, F-180).

## Finding 6: the server going away gives exit status 4 or 1

- Kind: competing behaviour
- Items: F-104, F-168, F-169
- Code: `cli.go:1555`, `internal/rex/actions.go:1116`, `internal/rex/client.go:116`

When the server ends, `rex events` exits with 4 ("server unreachable").
`rex wait`, waiting on the same server, exits with 1.

- Status: decided: the server going away exits with 4 for every command.

## Finding 7: two ways to match a client

- Kind: competing behaviour
- Items: F-113, F-193, F-175
- Code: `cli.go:1525`, `internal/rex/actions.go:1143`

`rex do -C` matches a client by the server's rules (F-113): full ID,
name in any case, position, end of ID of at least 4 characters. `rex
client inspect` matches in `rex` itself: full ID, an end of ID of any
length, or the exact name. `rex client inspect 1` matches every client
whose ID ends in `1`, and prints the first.

- Status: resolved (2026-11-09): `rex client inspect` asks the server, which finds the app as `rex do -C` does (F-193, T-109).

## Finding 8: one fact, two JSON names

- Kind: competing behaviour
- Items: F-130, F-166, T-107, T-108, T-124
- Code: `internal/rex/proto.go:165`, `internal/rex/proto.go:170`, `internal/rex/model.go:76`

`rex block ls --json` prints `exitCode`, `lastOutput` and `lastInput`.
`rex list --json`, `rex wait --json` and the model print `exit_code`.
Every other key is in snake case, as Rex's are.

- Status: decided: snake case everywhere: `exit_code`, `last_output`, `last_input`.

## Finding 9: on Windows, REX_BIN_DIR names a directory without rex

- Kind: competing behaviour
- Items: F-118, F-119, F-186
- Code: `internal/rex/server.go:219`, `internal/rex/block.go:239`, `skill.go:360`

Every block gets `REX_BIN_DIR` and that directory first in `PATH`. The
server makes the `rex` link there on Linux and macOS only. On Windows the
directory is empty, so Claude Code's hooks (`"$REX_BIN_DIR/rex"`) do
nothing, and `rex` is not on a block's `PATH` unless installed elsewhere.

- Status: resolved: GoRex is Linux only (D-16).

## Finding 10: commands with several targets stop half done

- Kind: difference from Rex
- Items: F-113, F-140
- Code: `cli.go:742`, `cli.go:962`, `cli.go:1279`

Rex's documentation: an ambiguous target changes nothing. `rex kill a b
c`, `rex close a b c` and `rex window close a b c` act on each target in
turn. When `b` is ambiguous, `a` is already gone.

- Status: decided: resolve every target first; IF one fails, THEN nothing changes and `rex` exits with 3.

## Finding 11: rex sets REX_SERVER and does not read it

- Kind: difference from Rex
- Items: F-106, F-118, F-120
- Code: `internal/rex/block.go:253`, `cli.go:405`, `internal/rex/proto.go:229`

Blocks get `REX_SERVER=unix:SOCKET`, as Rex's do. `rex` finds its server
by `GOREX_DIR` instead, and refuses `-S`, whose default in Rex is
`REX_SERVER`. A block whose `GOREX_DIR` is removed reaches another
server than the one `REX_SERVER` names.

- Status: decided: `rex` reads `REX_SERVER` (`unix:PATH`) before `GOREX_DIR`.

## Finding 12: GOREX_SESSION holds a block's ID

- Kind: inconsistent terms
- Items: F-109, F-118
- Code: `internal/rex/block.go:258`, `cli.go:437`

`GOREX_SESSION` is the block's ID, from the first version, where a
"session" was one terminal. `rex` still reads it when `REX_BLOCK` is
empty.

- Status: decided: `GOREX_SESSION` goes; `REX_BLOCK` is the one name.

## Finding 13: block, pane and terminal; window and tab

- Kind: inconsistent terms
- Items: glossary (block, window, close), F-140
- Code: `help.go:10`, `skill.go:193`, `commands.go` (action names), `README.md`

Rex's CLI and the model say session, window and block. The app and the
README say tab and pane, and the action names are Rex's own
(`pane.split`, `client.tab.new`). `rex help` says "tabs (windows) of
terminals (blocks)"; the skill says "a block is one terminal (a pane)".
The spec takes Rex's words: window (avoid tab), block (avoid pane).
`rex kill` ends sessions and `rex close` closes blocks; the spec says
close, and avoids kill.

- Status: decided: the CLI, its help and the skill say session, window and block; the app shows a window as a tab and a block as a pane, as Rex's app does. The glossary says so.

## Finding 14: the data directory on Linux

- Kind: inconsistent terms
- Items: F-106, T-120
- Code: `README.md:167`, `AGENTS.md:35`, `internal/rex/proto.go:237`, `main.go:69`

The README says `~/.config/gorex` on Linux. `AGENTS.md`, `wide.go` and
the code say `GoRex`: `rex.Dir()` joins the user's configuration
directory and `GoRex`, and the app sets `GOREX_DIR` from MyGo's data
directory of the app.

- Status: decided: the code is right (`GoRex`); the README is corrected.

## Finding 15: help that says less, or other, than the code

- Kind: other
- Items: F-103, F-169, F-178, F-186
- Code: `help.go:83`, `help.go:40`, `help.go:46`, `keymap.go:269`
- Status: in part resolved by D-13 (proposed): the help lists every flag
  from its declaration, and a group prints its own help. The prose
  below is still written by hand, and still says less.

- `rex help events`: without `-s`, only "sessions created and
  destroyed". The code also prints every session's window and block
  events.
- `rex help skill`: "a SessionStart hook". Setup adds seven hooks.
- `rex help keymap`: presets `linux`, `macos`, `none`. On macOS the
  default is `darwin`.
- `rex session` without a subcommand prints `help["session"]`, which
  does not exist, so the overview prints. Resolved: it prints the
  group's help.

## Finding 16: global options work only by the name rex

- Kind: accidental behaviour
- Items: F-100, F-101
- Code: `main.go:38`, `cli.go:99`

`GoRex ls` runs `rex ls`. `GoRex --timeout 5s ls` starts the app, as
`--timeout` is not a command. `rex --timeout 5s ls` works.

- Status: resolved (2026-11-09): the global options also work by the name GoRex, before a command of `rex` (F-100).

## Finding 17: flags taken and ignored

- Kind: accidental behaviour
- Items: F-101, F-161, F-164, F-176
- Code: `cli.go:469`, `cli.go:470`, `cli.go:191`, `cli.go:331`, `cli.go:1208`

`--color`, `-v`, `capture --unwrap`, `send-key --action repeat`, and `do
--origin`, `--for`, `--all`, `--fail-fast`, `-a` are taken so that Rex's
command lines run. They change nothing. Rex's scripts then get
different results with exit status 0.

Changed by D-13 (proposed): a flag declared as ignored says so in the
help and the reference ("taken, and ignored"). They still change
nothing.

- Status: decided (2026-11-09): kept, so that Rex's command lines run; the help and the reference say each one is ignored (D-13).

## Finding 18: Rex's flags that GoRex refuses with exit status 2

- Kind: difference from Rex
- Items: F-156, F-179, F-180, F-182, F-183
- Code: `cli.go:1767`, `cli.go:1856`, `cli.go:1487`, `cli.go:1881`

`rex config check --json`, `rex server stop --discard` and `--if-idle`,
`rex client ls -q` and `--short` exit with 2.

Corrected while making the reference from declarations (D-13): `rex
config reload --json` and `rex server logs --follow` and `-n` do not exit
with 2. Those commands read no flags, and take any argument and ignore
it, as Finding 17's flags. The old reference listed them as "Flags:
none", which read as "refuses every flag". GoRex's `server stop --forget`
is Rex's `--discard` under another name.

- Status: resolved (2026-11-09): taken as Rex: `client ls -q` and `--short` (F-156), `server stop --discard` (and `--forget`, GoRex's old name) and `--if-idle` (F-182), `config check --json` (F-179), `server logs --follow` and `-n` (F-183), `config reload --json` (F-180).

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

- Status: decided, item by item: remote (`-S`, hosts, home) deferred to the phone feature (D-5); `rex api` and Lua stay out (D-7, D-4); scrollback about 64 MiB and `--unwrap` as Rex; exit 130 on an interrupt; `completion powershell` refused (Linux only), `nu` deferred; the Codex hook deferred; the event fields and `rex actions` stay as they are, as GoRex's.
- Status: resolved (2026-11-09), for the items to change: scrollback about 64 MiB (F-165); `capture` keeps a wrapped line's rows apart unless `--unwrap` (F-164); 130 on an interrupt (F-104); `completion powershell` refused, saying GoRex runs on Linux (F-124). The rest stays deferred or kept, as decided.

## Finding 20: rex status is GoRex's own

- Kind: difference from Rex
- Items: F-172, F-173
- Code: `cli.go:1910`

Rex's CLI reference has no `rex status`; Rex's programs print OSC 7501
themselves. GoRex adds it for hooks that have no terminal.

- Status: decided: kept, as GoRex's own command.

## Finding 21: what the tests cannot show

- Kind: untestable
- Items: F-100, F-107, F-119, F-121, F-184
- Code: `main.go:78`, `internal/rex/server.go:970`, `internal/rex/server.go:218`, `cli.go:161`, `internal/rex/server.go:111`

Running by the name `rex`, starting a server (it runs `os.Executable`,
the test binary), the `bin/rex` link, the wide event of a command, and a
server exiting after 10 idle minutes have no test. F-121 says `verify:
event`; the others need a test that builds the executable.

- Status: decided: these items say `verify: manual` with how, or `event`; a test that builds the executable may come later.

## Finding 22: the matrix shows nothing of a retro spec

- Kind: spec rules
- Status: resolved in the rules (the matrix shows proposed items too)
- Items: all
- Code: `tools/spec/main.go:599`, `tools/spec/main.go:604`

`spec matrix` lists only accepted business and functional items. Every
item of this pass is `proposed`, so the matrix is empty until the owner
accepts an area. Coverage of proposed items is visible only through
`spec show` or `spec query`.

## Finding 23: tables escape the budgets

- Kind: spec rules
- Status: resolved in the rules (tables count as words; length is a style note, not a budget)
- Items: F-118, F-163, T-104, T-105, T-110
- Code: `tools/spec/main.go:854`

`spec check` leaves tables out of the word count and the sentence check.
The tables of exhaustive cases that `spec/README.md` asks for are free,
so F-118 or T-105 hold far more than 150 words of content. The budget
then limits prose, not size.

## Finding 24: the sentence limit is 25, not 20

- Kind: spec rules
- Status: resolved in the rules (length is a style note (`spec check --style`), not a limit)
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

- Status: decided: kept as it is; the glossary's "rex command" reads well.

## Finding 26: avoided words have one meaning in the spec, more in the code

- Kind: spec rules
- Status: resolved in the rules (partly: a longer glossary term, as "tab bar", may contain an avoided word)
- Items: glossary (window, block, close)
- Code: `tools/spec/main.go:949`

`avoid:` is checked by word, not by meaning. With `tab` avoided for
window, the spec cannot name the app's tab bar, or Rex's `Ctrl+Tab`
outside backticks. The GUI area needs `tab` for what the app draws.

## Finding 27: no source for the repository's documents

- Kind: spec rules
- Status: resolved in the rules (`source: repo` for the README, docs and git history)
- Items: all
- Code: `tools/spec/main.go:828`

`source:` takes `owner`, `rex-docs`, `code` and `tests`. The README and
the git history are neither code nor tests; this pass marks them `code`.
A `repo-docs` source would show where an item rests on prose a person
wrote.

## Finding 28: a zoomed window's focus moves to a hidden block

- Kind: competing behaviour
- Items: F-142, F-143, F-144
- Code: `internal/rex/actions.go:779`, `internal/rex/actions.go:802`, `internal/rex/actions.go:817`

In a zoomed window, `rex focus left` (or `right`, `up`, `down`) does not
move the focus. `rex focus next` and `prev` do: the focus goes to a block
the zoom hides, and the zoom stays on the other one. `rex focus BLOCK`,
for the same block, ends the zoom. Two ways to focus one block leave the
window in two states.

- Status: decided: focusing another block ends the window's zoom, by any command; directions and next/prev then move as without zoom.

## Finding 29: rex client inspect takes no flags

- Kind: difference from Rex
- Items: F-193
- Code: `cli.go:298`, `cli.go:1583`

`rex client inspect` reads its arguments without parsing flags. `rex
client inspect --json`, which Rex's reference shows, looks for a client
named `--json` and exits with 3. Without a client it takes the first app;
Rex takes `-C`, then `REX_CLIENT`, then the app when exactly one is
connected. Rex also prints the actions the client offers.

- Status: resolved (2026-11-09): it takes `--json`, `-C` and `REX_CLIENT`, and prints the app's actions, as Rex's reference says (F-193).

## Finding 30: small accidents of the layout commands

- Kind: accidental behaviour
- Items: F-132, F-142
- Code: `cli.go:522`, `internal/rex/actions.go:444`

- `--cwd ~bob/src` becomes the user's home followed by `bob/src`, not
  bob's home.
- `rex window focus NEXT` matches `next` in any case, then goes to the
  previous window: only the exact word `next` goes forward.

- Status: decided: `~user` is that user's home; `next` and `prev` match in any case, as other words.

## Finding 31: what the functional template does not hold

- Kind: spec rules
- Status: resolved in the rules (shared items, commas in conditions, several `superseded-by:`, `conflicts:` on one side)
- Items: F-132, F-133, F-137, F-148, F-112, F-113
- Code: `tools/spec/shape.go:29`

- Options that four commands share (F-132), the way a command starts
  (F-133) and the words for directions (F-148) are no one event's use
  case. Repeated in each item they say one thing four times; they stay
  items of their own, read by the items that use them.
- An EARS condition cannot hold a comma: `spec check` reads the first
  comma as its end. "WHEN `rex run`, `rex split` or …" fails; the rules
  say "`rex run` or `rex split` or …" instead.
- `superseded-by:` takes one item. F-137 went to two (F-134, F-135) and
  to T-124; the item's text says so.
- `conflicts:` on both sides makes a rewrite of one file change items of
  another (F-112, F-113 in `cli.md`).

## Finding 32: defaults go to a reference that has none

- Kind: spec rules
- Items: F-132, F-135, F-145, F-147
- Code: `reference_test.go`
- Status: resolved by D-13 (proposed): the reference lists each flag's
  default from its declaration, and says in words the defaults that a
  command computes, as the directory of F-132.

The template says a default is not a rule: the reference lists it. The
reference lists each command's flags and their spellings, not their
defaults, so `--ratio` 50, `--amount` 5 and `--split` right are now
written nowhere but the code and the help. The directory a new block
starts in is also a default, but it differs from command to command
(Finding 3) and the owner is asked about it (question 14), so F-132
keeps it, as a table. `conflicts:` cannot say that one item's table
holds competing behaviour; Finding 3 does.

## Finding 33: --autostart=VALUE is off for any value but true

- Kind: accidental behaviour
- Items: F-101, F-102, F-107
- Code: `cli.go:478`

`--autostart=1` turns the server's start off, and `--autostart=yes`
too, without an error. The flags of a command take `1` as true and
refuse `yes` with exit status 2 (F-102). D-13 kept the difference: the
owner decides whether `--autostart` is a boolean as the others.

- Status: resolved (2026-11-09): `--autostart` is a boolean as every other flag: true, false, 1, 0; anything else exits with 2 (F-101).

## Finding 34: an unknown capture format exits with 1, html with 2

- Kind: competing behaviour
- Items: F-164, F-104
- Code: `cli.go:1246`, `internal/rex/block.go:531`

`rex capture --format html` exits with 2: `rex` refuses it. `rex
capture --format foo` goes to the server, which refuses it with an
error that is not a usage error, so `rex` exits with 1, and only when a
server answers. A wrong value is a wrong command line (F-104: 2). D-13
did not declare the formats as the values `--format` takes, which would
make both exit with 2 before connecting.

- Status: resolved (2026-11-09): `--format` declares its values, `text` and `vt`; another, `html` too, exits with 2 before connecting (F-164).

## Finding 35: the server clamps a ratio in three ways

- Kind: competing behaviour
- Items: F-135, F-145, F-147
- Code: `internal/rex/actions.go` (0.05 to 0.95, and 0.1 to 0.9), `view.go` (0.08 to 0.92)

`rex` takes `--ratio` from 1 to 99 (D-14), as Rex does. The server then
holds a split's ratio to 5–95 % in one place and 10–90 % in another, and
the app's view to 8–92 %. `--ratio 3` gives 5 %, without a word. Found by
the percentage change of the experiment (`p2-old-pct`).

- Status: decided: one limit, 5 to 95 %, kept in one place and used by the server and the app; `rex` takes 1 to 99 and the server holds it in the limit.
