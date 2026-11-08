# rex: invocation, targets, output, exit statuses, environment

The commands themselves are in `cli-layout.md`, `cli-terminals.md` and
`cli-app.md`.

## Invocation

### F-100 Run as rex
status: proposed · traces: B-3, B-7 · source: code

The executable SHALL run as `rex` when its file name, without `.exe` and
in any case, is `rex`.
WHEN it runs by another name, and its first argument is a command of
`rex`, THEN it SHALL run as `rex` too.
WHEN `rex` gets no arguments THEN it SHALL print the overview help and
exit with 0.

### F-101 Global options
status: proposed · traces: B-3, B-5 · source: code, rex-docs

Before the command, `rex` SHALL take these options:

| Option | Effect |
|---|---|
| `-C`, `--client CLIENT` | the app that `rex do` asks; default `REX_CLIENT` |
| `--timeout DURATION` | the longest wait for a request's answer; default `10s` |
| `--autostart[=false]` | start a server when none answers; default on |
| `--color WHEN` | taken and ignored |
| `-v`, `--verbose`, `-vv` | taken and ignored |
| `-S`, `--server` | refused, with exit status 2 |

WHEN an option before the command is not in the table THEN `rex` SHALL
exit with 2. A bad `--timeout` value SHALL exit with 2.

### F-102 Flags of a command
status: proposed · traces: B-3 · source: code

`rex` SHALL take a command's flags before, between and after its
arguments, as `--name value`, `--name=value` or `-n value`.
A boolean flag SHALL take no value, or `=true`, `=false`, `=1` or `=0`.
Everything after `--` SHALL be the command to run, not flags.
An argument that is a number, such as `-5`, SHALL be an argument.
WHEN a flag is unknown, lacks its value, or a boolean has another value,
THEN `rex` SHALL exit with 2.

### F-103 Help
status: proposed · traces: B-3 · source: code

`rex help`, `rex --help` and `rex -h` SHALL print the overview: the
commands, the target rules and the exit statuses.
`rex help COMMAND` SHALL print the help of `split`, `run`, `capture`,
`send-key`, `skill`, `keymap`, `status`, `do`, `events` or `wait`.
For any other topic it SHALL print the overview.
A group without a subcommand, such as `rex session`, SHALL print help
and exit with 0.
WHEN the command or subcommand is unknown THEN `rex` SHALL exit with 2.

## Exit statuses and errors

### F-104 Exit statuses
status: proposed · traces: B-3, B-5 · source: code, rex-docs

`rex` SHALL exit with:

| Status | When |
|---|---|
| 0 | the command did what it was asked |
| 1 | it failed for another reason |
| 2 | the command line is wrong, or the request makes no sense |
| 3 | a target matches nothing, or more than one thing |
| 4 | no server answers |
| the program's | `rex wait`, and `--wait`, for a block's program |

WHEN a command fails THEN `rex` SHALL print `Error: ` and the reason on
standard error.

### F-105 The exit status of a program waited for
status: proposed · traces: B-3 · source: code, rex-docs

WHEN `rex` waits for a block's program THEN it SHALL exit with that
program's exit status, modulo 256.
A program that a signal ended SHALL count as 128 plus the signal's
number.
WHEN the program exited with 0 THEN `rex` SHALL exit with 0 and print
nothing on standard error.

## The server

### F-106 Which server
status: proposed · traces: B-6 · source: code

`rex` SHALL talk to the server of the data directory: `GOREX_DIR`, else
the user's configuration directory and `GoRex`.
The socket SHALL be `server.sock` in the data directory.
WHEN that path has 100 characters or more THEN the socket SHALL be
`gorex-UID-HASH.sock` in the system's temporary directory.

### F-107 Start a server when none runs
status: proposed · traces: B-2, B-3 · source: code, rex-docs

WHEN no server answers THEN `rex` SHALL start one in the background and
wait up to 5 seconds for it.
WHEN `--autostart=false` is given and no server answers THEN `rex` SHALL
exit with 4 and start nothing.
WHEN the server still does not answer THEN `rex` SHALL exit with 4.
`rex version`, `rex server status` and `rex server stop` SHALL NOT start
a server.

### F-108 A server of another version
status: proposed · traces: B-2 · source: code

WHEN the server speaks another protocol version than `rex` THEN `rex`
SHALL exit with 1.
Its message SHALL name both versions and tell the user to open GoRex,
which restarts the server.

## Context and targets

### F-109 Where rex runs
status: proposed · traces: B-3, B-4 · source: code, rex-docs

`rex` SHALL take its context from `REX_SESSION` and `REX_BLOCK`.
WHEN `REX_BLOCK` is empty THEN `rex` SHALL take the block from
`GOREX_SESSION`.
A command that names no target SHALL act in its context (F-110, F-111,
F-112).

### F-110 The session a command acts in
status: proposed · traces: B-3, B-4 · source: code

The session SHALL be the first of:

1. the session that `-s` names;
2. the context session;
3. the session of the context block;
4. the session that an app showed last;
5. the first session.

WHEN there is no session THEN `rex` SHALL exit with 3.

### F-111 The window a command acts in
status: proposed · traces: B-3, B-4 · source: code

WHEN `-w` names a window THEN the command SHALL act in it.
WHEN neither `-s` nor `-w` is given THEN it SHALL act in the window of
the context block.
Else it SHALL act in the active window of the session of F-110.

### F-112 The block a command acts on
status: proposed · traces: B-3, B-4 · source: code, rex-docs

WHEN `-b`, or the block argument, names a block THEN the command SHALL
act on it.
WHEN no `-s` and no `-w` is given THEN it SHALL act on the context block.
Else it SHALL act on the focused block of the window of F-111, or its
first block.

### F-113 How a target matches
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

`rex` SHALL match a target in this order, and stop at the first kind
that matches:

1. a full ID;
2. a label, in any case;
3. a position from 1, for sessions and windows;
4. the end of an ID, of at least 4 characters, without a leading `…`.

WHEN a target matches nothing THEN `rex` SHALL exit with 3.
WHEN a kind matches more than one item THEN `rex` SHALL change nothing,
list the matches with short ID and name, and exit with 3.

### F-114 Where a block target matches
status: proposed · traces: B-3 · source: code, tests

WHEN `-w` is given THEN a block target SHALL match in that window only.
WHEN `-s` is given THEN it SHALL match in that session only.
Else it SHALL match in the session of F-110, then in every session.
WHEN it is ambiguous in the first of these THEN `rex` SHALL stop there.

### F-115 Where a window target matches
status: proposed · traces: B-3 · source: code

A window target SHALL match in the session of F-110.
A full window ID SHALL also match a window of any other session.

## Output

### F-116 Tables and JSON
status: proposed · traces: B-3, B-5 · source: code, rex-docs

By default, listings SHALL print a table for people, with short IDs.
`--short=false` SHALL print full IDs.
`-q` or `--quiet` SHALL print the IDs alone, one a line.
`--json` SHALL print indented JSON with full IDs.
A command that creates something SHALL print its ID on a line.

### F-117 Marks in listings
status: proposed · traces: B-4 · source: code, tests

| Listing | Mark | Meaning |
|---|---|---|
| `rex ls` | `*` | the context session |
| `rex window ls`, `rex session inspect` | `*` | the active window |
| `rex block ls` | `@` | the context block |
| `rex block ls` | `*` | a focused block, not the context block |
| `rex list` | `*`, `>` | the active window, the focused block |

## Environment

### F-118 The environment of a block
status: proposed · traces: B-3, B-4 · source: code, rex-docs

The server SHALL start each block's program with the server's
environment, without the variables of other terminals, and with:

| Variable | Value |
|---|---|
| `REX_SESSION` | the block's session ID |
| `REX_BLOCK` | the block's ID |
| `REX_SERVER` | `unix:` and the socket's path |
| `REX_BIN_DIR` | `bin` in the data directory |
| `GOREX_DIR` | the data directory |
| `GOREX_SESSION` | the block's ID |
| `PATH` | `REX_BIN_DIR`, then the server's `PATH` |
| `TERM`, `COLORTERM` | `xterm-256color`, `truecolor` |
| `TERM_PROGRAM`, `TERM_PROGRAM_VERSION` | `GoRex`, its version |
| `LANG` | `en_US.UTF-8`, when no `LANG`, `LC_ALL` or `LC_CTYPE` is set |

### F-119 The rex of a block's PATH
status: proposed · traces: B-4, B-5 · source: code, rex-docs

The server SHALL keep `rex` in `REX_BIN_DIR` as a link to its own
executable, on Linux and macOS.
A block that runs `rex` SHALL then run the `rex` of its own server.

### F-120 What rex reads from the environment
status: proposed · traces: B-3 · source: code

`rex` SHALL read `REX_SESSION`, `REX_BLOCK` and `GOREX_SESSION` (F-109),
`REX_CLIENT` (F-101), `GOREX_DIR` (F-106) and `GOREX_EVENTS` (F-121).
`rex` SHALL NOT read `REX_SERVER`.

### F-121 A wide event per command
status: proposed · traces: B-8 · verify: event · source: code

After each command but `help` and `completion`, `rex` SHALL append one
wide event of kind `cli`.
It SHALL hold the command's words, the names of its flags, its exit
status, its duration, and its context.
It SHALL NOT hold the values of flags or the text of arguments.
It SHALL name the agent when `CLAUDECODE`, `CODEX_SANDBOX` or `CODEX_HOME`
is set.
WHEN `GOREX_EVENTS` is `off` or `0` THEN `rex` SHALL record nothing.

## Other

### F-122 GoRex's first commands
status: proposed · traces: B-3 · source: code

`rex` SHALL keep these forms of its first version:

| Form | Is |
|---|---|
| `rex list` | the tree of sessions, windows and blocks (F-153) |
| `rex new-tab [LABEL] [CMD...]` | `rex window new` |
| `rex send-keys [BLOCK] TEXT` | `rex send`, with `<CR>`, `<Esc>`, `\n`, `\r`, `\e` replaced |
| `rex split -t BLOCK` | `-b BLOCK` |
| `rex split -v`, `--vertical`, `--down`, `--below` | `--split=below` |
| `rex split --horizontal`, `--right`, `--beside` | `--split=right` |
| `rex split CMD...`, without `--` | the command `CMD...` |

### F-123 Version
status: proposed · traces: B-3 · source: code

`rex version` SHALL print `rex VERSION (GoRex, protocol N)`.
WHEN a server runs THEN it SHALL also print the server's process ID and
protocol.
`--json` SHALL print `version`, `protocol`, `name`, and `server` when one
runs.

### F-124 Shell completion
status: proposed · traces: B-3 · source: code

`rex completion bash`, `zsh` or `fish` SHALL print a script that completes
the first word with the commands of `rex`.
Another shell SHALL exit with 2.
