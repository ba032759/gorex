# Decisions: architecture

### D-1 The server owns the model

status: accepted · traces: B-2, B-3 · source: code, repo

**Context.** The first versions kept the layout of windows and blocks in
the app (`layout.json`). The app served a socket of its own for the first
`rex` commands (commit `b6f4593`, `ipc.go`).

**Options.** (a) The app owns the layout and the server only runs the
programs. (b) The server owns sessions, windows, layouts, labels, focus
and zoom; every client mirrors this model and sends its changes as
requests.

**Choice.** (b), in commit `33c6ea8`, which removed `ipc.go`.

**Reason.** `rex` and the app then see the same thing, and `rex` works
without the app (README). With (a), a script had nothing to act on while
the app was closed.

**Consequences.** Every change goes through the server and raises the
model's revision. The app keeps no layout of its own. The first
`layout.json` becomes a session once.

### D-2 Follow Rex's command line

status: accepted · traces: B-5 · source: code, rex-docs

**Context.** GoRex had its own first commands: `list`, `new-tab`,
`send-keys`, and `split` with `-t` and `-v`.

**Options.** (a) Keep GoRex's own commands. (b) Take Rex's commands,
flags, target rules and exit statuses.

**Choice.** (b), keeping the first commands as aliases (D-3).

**Reason.** The source code states the goal, "the same commands, flags,
targets and exit statuses", and not its reason (B-5).

**Consequences.** What Rex has and GoRex does not is refused with a
message, or accepted and ignored. `spec/FINDINGS.md` lists the
differences.

### D-3 Keep GoRex's first commands working

status: accepted · traces: B-3 · source: code

**Context.** GoRex had commands of its own before it took Rex's (D-2):
`rex list`, `rex new-tab`, `rex send-keys`, `split -t`, `split -v`,
`split --horizontal`, a command without `--`, `rex close` without
arguments.
**Options.** (a) Remove them when Rex's commands came. (b) Keep them
working as they did, beside Rex's.
**Choice.** (b).
**Reason.** Scripts written for them keep working. The code says only
"GoRex's first …".

### D-4 No Lua

status: accepted · traces: B-5 · source: code, rex-docs

**Context.** Rex is configured in Lua (`init.lua`) and runs Lua scripts
and handlers with `rex do`.
**Options.** (a) An embedded Lua runtime, as Rex has. (b) No Lua:
configuration in `settings.json` and `keymap.json`, scripts in the shell,
and `rex do` runs only the app's actions.
**Choice.** (b).
**Reason.** Unknown.

### D-5 A local server only

status: accepted · traces: B-6 · source: code, rex-docs, repo

**Context.** Rex's server can listen for other machines: remote hosts,
home servers, Tailscale, shared and login servers, and `rex -S`.
**Options.** (a) Listen for other machines, as Rex does. (b) Listen on
one Unix socket in the data directory, readable by the user alone, and
refuse `-S`.
**Choice.** (b). README: "GoRex's server listens on a local socket only".
**Reason.** Unknown. Access from a phone needs (a) or a part of it.

### D-6 One executable, which starts its own server

status: accepted · traces: B-2 · source: code, rex-docs

**Context.** The app, the server and `rex` must be the same version: a
`rex` of another version may not speak the server's protocol.
**Options.** (a) A separate `rex` program. (b) One executable: the server
is the app's executable run with `-server`, and `rex` is that executable
by another name.
**Choice.** (b). Any client starts the server in the background when none
answers. The server links `bin/rex` in the data directory to its own
executable, and each block has that directory first in its `PATH`.
**Reason.** The `rex` in a block is always the one of its server. With
(a), a shell profile can put another version first in `PATH`, as Rex's
documentation warns. One file to install.

### D-7 A protocol of GoRex's own

status: accepted · traces: B-6 · source: code, rex-docs, owner

**Context.** Rex's server has an API (`rex api`, its dotted methods and
JSON schemas).
**Options.** (a) Rex's API. (b) JSON lines over the Unix socket, with
GoRex's own requests (`technical/protocol.md`).
**Choice.** (b).
**Reason.** GoRex keeps its own server (D-11), and the code took no
part of Rex's API.
**Consequences.** Rex's own `rex` cannot drive a GoRex server, and the
reverse.

### D-8 The server keeps each block's screen

status: accepted · traces: B-2 · source: code

**Context.** An app that attaches, and `rex capture`, need the screen of
a block whose program printed while no app showed it.
**Options.** (a) Replay the bytes the program printed, as the first
server did. (b) Feed each block's output to a terminal emulator without
a view (libghostty-vt), and send a snapshot of the screen and scrollback
at the app's size.
**Choice.** (b). `rex capture` reads the same.
**Reason.** Full-screen programs do not survive (a).

### D-9 Restore shells, not commands

status: accepted · traces: B-2 · source: code

**Context.** When the server ends, as at a reboot, its blocks' programs
end. A new server finds the saved layout.
**Options.** (a) Run each block's command again. (b) Start a shell in
each saved block, in the directory the block had, with the same IDs,
labels and layout.
**Choice.** (b).
**Reason.** From the code: a command "could do what it did once more, as
a deploy".

### D-10 Wide events for observation

status: accepted · traces: B-8 · source: code

**Context.** The owner and agents need a record of real use (B-8) that
holds no secrets.
**Options.** (a) Free-form logs. (b) Telemetry sent to a service. (c)
One flat JSON line per request, event and command, appended to `events/`
in the data directory, for DuckDB.
**Choice.** (c). The lines keep the names of flags, not their values, and
lengths of text, not the text.
**Reason.** An agent queries (c) with SQL; (a) must be parsed. (b) sends
data off the machine. Rex sends no telemetry either.

### D-11 GoRex keeps its own server, not Rex's Linux server

status: accepted · traces: B-1 · source: owner, rex-docs

**Context.** Superlogical ships a Rex server for Linux and FreeBSD: one
`rex` binary that is the server and the CLI, without an app. It is closed
source, it is in testing, and it has no stable release.

**Options.** (a) GoRex becomes an app for Rex's server and removes its
own server. (b) GoRex keeps its own server.

**Choice.** (b), for now.

**Reason.** With (a), GoRex depends on a closed program in testing, whose
protocol GoRex cannot change and which can change at any time. GoRex also
needs to change its server: access from a phone, wide events. With (b),
GoRex is also a place to try how a spec drives work on a server.

**Consequences.** When Rex's Linux server has a stable release and a
documented API, examine (a) again. GoRex then gives a value only as the
app, and its own server is a cost.

### D-12 A native app, without a web view

status: accepted · traces: B-1 · source: repo, code

**Context.** A terminal app needs a window, text and input on Linux
(B-7).
**Options.** (a) A web view, as Electron apps have. (b) A native
interface (MyGo) and a terminal emulator in the same executable
(libghostty-vt), without cgo.
**Choice.** (b). The executable is about 15 MB.
**Reason.** Unknown. The README names it as a feature.

### D-13 Declare rex's commands with Go's standard library

status: accepted · traces: B-3, B-5 · source: code, owner

**Context.** A command of `rex` lived in five places: the list of
commands, its handler's flags, the defaults in its logic, `help.go` and
the completion. The help left out 13 flags of 3 commands (Finding 15).
The reference ran each handler to find its flags, could not run six of
them, and listed no defaults (Finding 32).

**Options.** (a) The hand-written parser, as it was. (b) A library, such
as kong or cobra. (c) One declaration per command and per flag, in Go's
standard library only: a parser of GoRex's own that reads declarations
in the shape of the `flag` package's. (d) The `flag` package's
`FlagSet` as the parser.

**Choice.** (c).

**Reason.** With (c), a command and its flags are declared once, and the
help, the completion and `spec/reference/cli.md` read the declarations:
a flag cannot be missing from its help, and the reference lists every
default without running a command. (a) keeps the copies. (b) adds a
dependency to `go.mod` and a syntax of its own; branch `cli-kong`
measures it. (d) does not fit Rex's command line: `FlagSet` stops at the
first argument, does not tell whether `--` ended the flags, reads `-5`
as a flag, takes `t`, `F` and `TRUE` as booleans, answers `-h` itself,
makes each spelling a flag of its own, and its durations lose the
message of `time.ParseDuration`.

**Consequences.** A flag is added in one place, its declaration. The
declared default is the value that `rex` sends: `--ratio` 50 and
`--amount` 5 now go to the server, which had the same defaults, so its
events show `arg_ratio` 0.5 where they showed 0. The two ranges of
`--ratio` stay (Finding 1); the help now shows both. Look at the choice
again when `rex` needs a syntax the parser lacks, as `-qA` for `-q -A`.

### D-14 Percentages are percent, as Rex's

status: rejected · traces: B-5 · source: rex-docs, owner
rationale: Rex's reference takes `--ratio PCT` as percent, from 1 to 99;
a fraction would make `0.5` mean another thing than in Rex.

**Context.** A change tried on branch `p2-stdlib-pct` (the experiment of
`docs/experiments/cli-declared.md`) let every percentage flag also take a
fraction: `0.3` is 30 %.
**Options.** (a) Percent only, `30` or `30%`. (b) Percent or a fraction.
**Choice.** (a). One range for every `--ratio`, from 1 to 99, as Rex's
reference gives it; the error says the range.
**Reason.** Rex's documentation. Of the change, what stays: one place
parses a percentage, the message comes from the declared range, and NaN
is refused.

### D-15 No --quiet for rex split and rex run

status: rejected · traces: B-5 · source: rex-docs, owner
rationale: in Rex, `-q`, `--quiet` means "print IDs only", on the
listing commands; split and run already print only the new block's ID.

**Context.** A change tried on branches `p2-*-quiet` gave `rex split` and
`rex run` a `-q` that prints nothing. The three tries did not agree on
`-q --json`.
**Options.** (a) Add it. (b) Leave it out.
**Choice.** (b).
**Reason.** Rex's reference. A second meaning of `-q` breaks "one flag,
one meaning", and scripts can send the ID to `/dev/null`.

### D-16 Linux only: no code for macOS and Windows

status: accepted · traces: B-7 · source: owner

**Context.** GoRex was built for Linux, macOS and Windows; `mise run
check` built it for macOS and Windows too. The owner has a Linux laptop
only, so the code for the other systems runs nowhere it is tested
(Finding 9).
**Options.** (a) Keep the code for the other systems, built and not run.
(b) Remove it: GoRex builds for Linux only.
**Choice.** (b).
**Reason.** Code that nobody runs drifts without notice; a build that
compiles tells nothing of what it does. Git keeps what is removed.
**Consequences.** `mise run check` builds for Linux only. Findings and
items about macOS and Windows are resolved or superseded. A port is a
decision of its own, with a system to test it on.
