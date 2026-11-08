# Decisions: architecture

### D-1 The server owns the model
status: proposed · traces: B-2, B-3 · source: code

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
status: proposed · traces: B-5 · source: code, rex-docs

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
status: proposed · traces: B-3 · source: code

**Choice.** `rex list`, `rex new-tab`, `rex send-keys`, `split -t`,
`split -v`, `split --horizontal`, a command without `--`, and `rex close`
without arguments still work as they did.

**Rejected.** Removing them when Rex's commands came. The code gives the
reason only as "GoRex's first …": scripts written for them keep working.

### D-4 No Lua
status: proposed · traces: B-5, B-7 · source: code, rex-docs
rationale: unknown

**Context.** Rex is configured in Lua (`init.lua`) and runs Lua scripts
and handlers with `rex do`.

**Choice.** GoRex has no Lua. Configuration is `settings.json` and
`keymap.json`; scripts are shell scripts; `rex do` runs only the app's
actions.

**Rejected.** An embedded Lua runtime. The sources give no reason.

### D-5 A local server only
status: proposed · traces: B-6 · source: code, rex-docs

**Choice.** The server listens on one Unix socket in the data directory,
readable by the user alone. `rex` refuses `-S`.

**Rejected.** Rex's remote hosts, home servers, Tailscale, shared and
login servers. README: "GoRex's server listens on a local socket only".
Why remote access is out is not stated.

### D-6 One executable, which starts its own server
status: proposed · traces: B-2, B-7 · source: code, rex-docs

**Choice.** The server is the app's executable run with `-server`. Any
client starts it in the background when none answers. The server links
`bin/rex` in the data directory to its own executable. Each block has
that directory first in its `PATH`.

**Reason.** The `rex` in a block is always the one that matches its
server. One file to install (B-7).

**Rejected.** A separate `rex` program: a shell profile puts another
version of it first in `PATH`, as Rex's documentation warns.

### D-7 A protocol of GoRex's own
status: proposed · traces: B-6 · source: code, rex-docs
rationale: unknown

**Choice.** Clients speak JSON lines over the Unix socket, with GoRex's
own requests (`technical/protocol.md`).

**Rejected, by omission.** Rex's server API (`rex api`, its dotted
methods and JSON schemas). The sources do not say why.

**Consequences.** Rex's own `rex` cannot drive a GoRex server, and the
reverse.

### D-8 The server keeps each block's screen
status: proposed · traces: B-2 · source: code

**Choice.** The server feeds each block's output to a terminal emulator
without a view (libghostty-vt). An app that attaches gets a snapshot of
the screen and scrollback at its size; `rex capture` reads the same.

**Rejected.** Replaying the bytes a program printed, as the first server
did, which full-screen programs do not survive.

### D-9 Restore shells, not commands
status: proposed · traces: B-2 · source: code

**Choice.** When a new server starts after one that ended, it starts a
shell in each saved block, in the directory the block had. IDs, labels
and the layout stay.

**Reason.** From the code: a command "could do what it did once more, as
a deploy".

### D-10 Wide events for observation
status: proposed · traces: B-8 · source: code

**Choice.** The server and `rex` append one flat JSON line per request,
event and command to `events/` in the data directory, for DuckDB.
They keep the names of flags, not their values, and lengths of text,
not the text.

**Rejected.** Free-form logs, and telemetry sent anywhere. Rex sends no
telemetry either.
