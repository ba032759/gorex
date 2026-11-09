# rex: invocation, targets, output, exit statuses, environment

What every command of `rex` shares. The commands themselves are in
`cli-layout.md`, `cli-terminals.md` and `cli-app.md`; the flags each
command takes, and their defaults, are in `reference/cli.md`. The
socket's path, the marks of listings, a block's environment and what
`rex version` prints are in `technical/cli.md` (T-140 to T-143).

## Invocation

### F-100 Run as rex
status: proposed · traces: B-3, B-7 · source: code, tests, owner

The executable SHALL run as `rex` when its file name, in any case, is
`rex`.
WHEN it runs by another name with a command of `rex` as its first word
after the options of F-101, the executable SHALL run as `rex` too.
WHEN `rex` gets no arguments, it SHALL print the overview help and exit
with 0.
IF the command or subcommand is not one of `rex`'s, THEN `rex` SHALL
exit with 2 and name `rex help`.

### F-101 Options before the command
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests, owner

`rex` SHALL take its options before the command, and read them as it
reads a command's flags (F-102).
IF an option before the command is not one of `rex`'s, THEN `rex` SHALL
exit with 2.
IF `-S` or `--server` is given, THEN `rex` SHALL exit with 2 and say
that GoRex's server runs on this machine (D-5).
`rex` SHALL take `--color`, `-v`, `--verbose` and `-vv`, so that Rex's
command lines run, and SHALL change nothing for them.
`--timeout` SHALL limit each connection to the server, a server started
included, and the wait for each answer.
WHILE `rex wait` or `--wait` waits for a program, or `rex events`
prints events, `--timeout` SHALL NOT end the wait.

`-C` names the app of `rex do` (F-175); `--autostart` is F-107's.

### F-102 Flags of a command
status: proposed · traces: B-3 · source: code, tests

`rex` SHALL take a command's flags before, between and after its
arguments, as `--name value`, `--name=value` or `-n value`.
A boolean flag SHALL take no value, or `=true`, `=false`, `=1` or `=0`.
`rex` SHALL take every word after `--` as the command to run, not as a
flag.
`rex` SHALL take a word that is a number, such as `-5`, as an argument.
WHEN the value of a flag is empty, `rex` SHALL use the flag's default.
IF a flag is unknown, lacks its value or has a value it does not take,
THEN `rex` SHALL exit with 2 and name the flag.

### F-103 Help
status: proposed · traces: B-3 · source: code, tests, owner

`rex help` and `rex --help` and `rex -h` SHALL print the overview: the
commands, the options before the command, the target rules and the exit
statuses.
`rex help COMMAND` SHALL print the help of a command or a group, as
`rex help session new`.
WHEN `--help` or `-h` follows a command, before `--`, `rex` SHALL print
that command's help, as `rex help COMMAND` does, and exit with 0.
WHEN a group runs without a command, as `rex session`, `rex` SHALL
print the group's help, which lists its commands, and exit with 0.
WHEN the topic of `rex help` is not a command, `rex` SHALL print the
overview and exit with 0.
The help of a command SHALL list every flag it takes, with its
spellings, the values it takes and its default (D-13).
The help SHALL say session, window and block, and SHALL use the app's
words for them only to say how the app shows a window and a block.

## Exit statuses and errors

### F-104 Exit statuses
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests, owner

`rex` SHALL exit with the status of this table:

| Status | When |
|---|---|
| 0 | the command did what it was asked |
| 1 | it failed for another reason |
| 2 | the command line is wrong, or the request makes no sense |
| 3 | a target matches nothing, or more than one thing |
| 4 | no server answers, or the server went away before it answered |
| the program's | `rex wait`, and `--wait`, for a block's program (F-105) |
| 130 | an interrupt, as `Ctrl+C`, ended the command |

WHEN a command fails, `rex` SHALL print `Error: ` and the reason on
standard error.

### F-105 The exit status of a program waited for
status: proposed · traces: B-3 · source: code, rex-docs, tests

WHEN a block's program that `rex` waits for exits, `rex` SHALL exit
with the program's exit status, modulo 256.
WHEN a signal ended the program, `rex` SHALL exit with 128 plus the
signal's number.
WHEN the program exits, `rex` SHALL print nothing on standard error,
whatever its exit status.

## The server

### F-106 Which server
status: proposed · traces: B-6 · source: code, rex-docs, tests, owner

WHEN `REX_SERVER` is `unix:PATH`, `rex` SHALL talk to the server whose
socket is `PATH`.
WHEN `REX_SERVER` is empty, `rex` SHALL talk to the server of the data
directory.
The server SHALL listen on the socket of its data directory, whatever
`REX_SERVER` says (T-140).
IF `REX_SERVER` is not empty and not `unix:PATH`, THEN `rex` SHALL exit
with 2.
IF `REX_SERVER` names another socket than that of the data directory
and no server answers on it, THEN `rex` SHALL exit with 4 and start no
server.

### F-107 Start a server when none runs
status: proposed · traces: B-2, B-3 · source: code, rex-docs, tests, owner

WHEN no server answers, `rex` SHALL start one in the background, and
wait for it up to `--timeout`.
IF `--autostart=false` is given and no server answers, THEN `rex` SHALL
exit with 4 and start nothing, except for `rex server start`, which asks
for a server (F-183).
IF the server started does not answer in time, THEN `rex` SHALL exit
with 4.
`rex version`, `rex server status` and `rex server stop` SHALL NOT
start a server.

The tests start the tests' executable, which is not a server: they show
that `rex` starts it and gives up after `--timeout`. The owner checks by
hand that a server starts: with `REX_SERVER=` and a new, empty
`GOREX_DIR`, `rex ls` exits with 0, and `rex server status` then tells
that a server runs.


### F-108 A server of another version
status: proposed · traces: B-2 · source: code

IF the server speaks another protocol version than `rex`, THEN `rex`
SHALL exit with 1.
The message SHALL name both versions and tell the user to open GoRex,
which offers to restart the server.

## Context and targets

### F-109 Where rex runs
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests, owner

`rex` SHALL take its context from `REX_SESSION` and `REX_BLOCK`.
`rex` SHALL NOT take its context from `GOREX_SESSION`.
WHEN a command names no target, it SHALL act in its context (F-110,
F-111, F-112).

### F-110 The session a command acts in
status: proposed · traces: B-3, B-4 · source: code, tests

The session SHALL be the first of:

1. the session that `-s` names;
2. the context session;
3. the session of the context block;
4. the session that an app showed last;
5. the first session.

IF there is no session, THEN `rex` SHALL exit with 3.

### F-111 The window a command acts in
status: proposed · traces: B-3, B-4 · source: code, tests

WHEN `-w` names a window, the command SHALL act in it.
WHEN neither `-s` nor `-w` is given, the command SHALL act in the window
of the context block.
WHEN `-s` is given without `-w`, the command SHALL act in the active
window of the session of F-110.

### F-112 The block a command acts on
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests, owner

WHEN `-b` or the block argument names a block, the command SHALL act on
it.
WHEN no block and no `-s` and no `-w` is given, the command SHALL act
on the context block.
WHEN `-s` or `-w` is given, the command SHALL act on the focused block
of the window of F-111, else on its first block.
IF the context block is not one of the server's, THEN the command SHALL
act on that block of the window of F-111 too.

### F-113 How a target matches
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests, owner

`rex` SHALL match a target in this order, and stop at the first kind
that matches:

1. a full ID;
2. a label, in any case;
3. a position from 1, for sessions and windows;
4. the end of an ID, of at least 4 characters, after the `…` that
   listings print before a short ID.

IF a target matches nothing, THEN `rex` SHALL change nothing and exit
with 3.
IF a kind matches more than one item, THEN `rex` SHALL change nothing,
list the matches with short ID and name, and exit with 3.
WHEN a command has several targets, `rex` SHALL resolve them all before
it acts on any.
WHEN two targets of a command name one item, `rex` SHALL act on it
once.

### F-114 Where a block target matches
status: proposed · traces: B-3 · source: code, tests

WHEN `-w` is given, a block target SHALL match in that window only.
WHEN `-s` is given without `-w`, a block target SHALL match in that
session only.
WHEN neither `-s` nor `-w` is given, a block target SHALL match in the
session of F-110, then in every session.
IF a block target matches more than one block in the session of F-110,
THEN `rex` SHALL exit with 3 and not look in the other sessions.

### F-115 Where a window target matches
status: proposed · traces: B-3 · source: code

A window target SHALL match in the session of F-110.
WHEN a window target is a full ID, it SHALL also match a window of any
other session.

## Output

### F-116 Listings and JSON
status: proposed · traces: B-3, B-4, B-5 · source: code, rex-docs, tests

`rex ls` and the other `ls` commands SHALL print a table for people,
with short IDs.
WHEN `--short=false` is given, a listing SHALL print full IDs.
WHEN `-q` or `--quiet` is given, a listing SHALL print the IDs alone,
one a line.
WHEN `--json` is given, a command SHALL print JSON with full IDs instead
of its text.
A listing SHALL mark the context session and the context block, the
active window and the focused block (T-141).
A command that creates something SHALL print the new item's ID on a
line.

### F-117 Marks in listings
status: superseded · superseded-by: F-116, T-141 · traces: B-4 · source: code, tests

That a listing marks the context is now a rule of F-116; the marks are
T-141.

| Listing | Mark | Meaning |
|---|---|---|
| `rex ls` | `*` | the context session |
| `rex window ls`, `rex session inspect` | `*` | the active window |
| `rex block ls` | `@` | the context block |
| `rex block ls` | `*` | a focused block, not the context block |
| `rex list` | `*`, `>` | the active window, the focused block |

## Environment

### F-118 The environment of a block
status: proposed · traces: B-3, B-4, B-5 · source: code, rex-docs, tests, owner

The server SHALL start each block's program with the server's
environment, without the variables of other terminals and of GoRex
(T-142).
The server SHALL set `REX_SESSION` and `REX_BLOCK` to the block's
session and block, and `REX_SERVER` to its own socket.
The server SHALL put `REX_BIN_DIR`, a directory that holds the server's
`rex`, first in the block's `PATH`.
WHEN a block's program runs `rex` from `PATH`, it SHALL run the `rex`
of the block's server, in the block's context.

### F-119 The rex of a block's PATH
status: superseded · superseded-by: F-118, T-142 · traces: B-4, B-5 · source: code, rex-docs

What a block's `rex` runs is now a rule of F-118; the link in
`REX_BIN_DIR` is T-142.

The server SHALL keep `rex` in `REX_BIN_DIR` as a link to its own
executable.
A block that runs `rex` SHALL then run the `rex` of its own server.

### F-120 What rex reads from the environment
status: superseded · superseded-by: F-106, F-109 · traces: B-3 · source: code, owner

Each variable is now said once, in the item that uses it: `REX_SERVER`
in F-106, `REX_SESSION` and `REX_BLOCK` in F-109, `GOREX_EVENTS` in
F-121, `REX_CLIENT` in F-175; `GOREX_DIR` is the data directory's
(glossary).

`rex` SHALL read `REX_SESSION` and `REX_BLOCK` (F-109), `REX_CLIENT`
(F-101), `REX_SERVER` and `GOREX_DIR` (F-106), and `GOREX_EVENTS`
(F-121).

### F-121 A wide event per command
status: proposed · traces: B-8 · verify: event · source: code, tests, owner

WHEN a command ends, `rex` SHALL append one wide event of kind `cli`,
except for `rex help` and `rex completion`.
The event SHALL hold the command's words, the names of its flags, its
exit status, its duration and its context.
The event SHALL NOT hold the values of flags, the text of arguments, or
a word that is not a declared command or flag.
IF the command line is not valid, THEN the event SHALL say `usage`, not
the message, which quotes the command line.
The command's words SHALL be the names of the command, as the tree of
commands declares them (`session new`), and never its arguments.
WHEN `CLAUDECODE` or `CODEX_SANDBOX` or `CODEX_HOME` is set, the event
SHALL name the agent that runs `rex`.
WHEN `GOREX_EVENTS` is `off` or `0`, `rex` SHALL record nothing.

T-121 lists the fields.

## Other

### F-122 GoRex's first commands
status: proposed · traces: B-3 · source: code, tests

`rex` SHALL keep these forms of its first version (D-3):

| Form | Is |
|---|---|
| `rex list` | the tree of sessions, windows and blocks (F-130) |
| `rex new-tab [LABEL]` | `rex window new` |
| `rex new-tab WORD WORD...`, `rex window new WORD WORD...` | `rex window new -- WORD WORD...`: two words or more are the command |
| `rex send-keys [BLOCK] TEXT` | `rex send`, with `<CR>`, `<Esc>`, `\n`, `\r`, `\e` replaced by their keys |
| `rex split -t BLOCK`, `--target BLOCK` | `-b BLOCK` |
| `rex split -v`, `--vertical`, `--down`, `--below` | `--split=below` |
| `rex split --horizontal`, `--right`, `--beside` | `--split=right` |
| `rex split CMD...`, without `--` | `rex split -- CMD...` |

### F-123 Version
status: proposed · traces: B-3 · source: code, tests

`rex version` SHALL print the version of `rex` and its protocol.
WHEN a server answers, `rex version` SHALL also print the server's
process ID and protocol.
`rex version` SHALL exit with 0 whether or not a server answers.

T-143 says what it prints.

### F-124 Shell completion
status: proposed · traces: B-3 · source: code, rex-docs, tests, owner

`rex completion bash` and `rex completion zsh` and `rex completion
fish` SHALL print a script that completes the commands of `rex`, the
commands of its groups, and the flags of each command.
IF the shell is `powershell`, THEN `rex` SHALL exit with 2 and say that
GoRex runs on Linux (D-16).
IF the shell is another or none is given, THEN `rex` SHALL exit with 2.

Rex also completes for `nu`; GoRex defers it.
