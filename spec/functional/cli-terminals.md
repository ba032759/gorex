# rex: type, read, wait, follow and report

The block these commands act on is that of F-112, or `-b`, `-s`, `-w`.

## Type

### F-160 Send text
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex send [TEXT...]` and `rex block send` SHALL type the text into the
block's terminal, as the user types it.
Several arguments, and those after `--`, SHALL be joined by one space.
WHEN no text is given THEN `rex` SHALL send what it reads from standard
input.
`rex` SHALL NOT add a newline or press Enter.
WHEN the block's program has exited THEN `rex` SHALL exit with 1.

### F-161 Press keys
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex send-key KEY...` and `rex block send-key` SHALL send the bytes of
each key, in order, as one input.
WHEN a key is not valid THEN `rex` SHALL send nothing and exit with 2.
WHEN no key is given THEN `rex` SHALL exit with 2.
`--action release` SHALL send nothing; `--action` with another value
SHALL press the keys.

### F-162 How keys are written
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

A key SHALL be modifiers and a key name, joined by `+`, in any case.
The modifiers SHALL be `shift`, `alt` (`opt`, `option`, `meta`), `ctrl`
(`control`) and `super` (`cmd`, `command`, `win`).
`ctrl++` SHALL be Control and plus.
A key name SHALL be a character, or a name of the table of F-163:
`enter`, `pageup`, `f12` and the others.
`[KeyA]`, `[Digit1]`, `[ArrowUp]`, `[Slash]` and others SHALL name a key
by its place on a US keyboard.
Aliases such as `return`, `esc`, `pgup`, `plus` and `comma` SHALL name
the same keys.

### F-163 The bytes of a key
status: proposed · traces: B-3 · source: code, tests

The server SHALL send xterm's bytes for each key:

| Key | Bytes |
|---|---|
| `enter`, `tab`, `escape` | CR, HT, ESC |
| `backspace`, `ctrl+backspace` | DEL, BS |
| `space`, `ctrl+space` | SP, NUL |
| `shift+tab` | `ESC [ Z` |
| a character with `ctrl` | its control character; none for `ctrl+1` |
| a character with `shift` | the character a US keyboard types |
| anything with `alt` | ESC before the key's bytes |
| arrows, `home`, `end` | `ESC [ A`…; `ESC O A`… when the program asked for application cursor keys |
| modified arrows | `ESC [ 1 ; M A`, M being 1 plus shift 1, alt 2, ctrl 4 |
| `insert`, `delete`, page keys, `f5`–`f12` | `ESC [ N ~`, with `; M` when modified |
| `f1`–`f4` | `ESC O P` to `ESC O S` |

WHEN `ctrl` and a key have no control character THEN the key SHALL be
not valid.

## Read

### F-164 Capture a screen
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests, owner

`rex capture` and `rex block capture` SHALL print the block's scrollback
and screen as text, a line for each row.
WHEN `--unwrap` is given, `rex` SHALL join the rows of a line that the
terminal wrapped.
`--trim` SHALL drop the spaces at the end of each line and the blank
lines at the end.
`--format vt` SHALL print the escape sequences that draw the screen.
IF `--format` is not `text` or `vt`, as `html`, THEN `rex` SHALL exit
with 2 before it connects to the server.
The output SHALL end with a newline.

### F-165 Scrollback
status: proposed · traces: B-2, B-3 · source: code, rex-docs, owner

The server SHALL keep about 64 MiB of each block's output above its
screen, as Rex does.
WHEN the scrollback is full, the server SHALL drop its oldest lines.
A full-screen program's screen SHALL have no scrollback.

## Wait

### F-166 Wait for a block, window or session
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex wait TARGET` SHALL match the target as a block, else a window, else
a session.
For a block, it SHALL wait for the program to exit, then exit with its
status (F-105).
For a window or session, it SHALL wait for it to close, then exit with 0.
`--json` SHALL print the result on one line: the target's ID and
`exit_code`.
WHEN no target is given THEN `rex` SHALL exit with 2.

### F-167 Wait for a block that closed
status: proposed · traces: B-3 · source: code, tests

The server SHALL keep the exit status of a closed block for 10 minutes.
WHEN `rex wait` names a block that closed in that time THEN it SHALL exit
with that status at once.
WHEN the block is gone and its status is not known THEN `rex` SHALL exit
with 1.

### F-168 Give up waiting
status: proposed · traces: B-3 · source: code, rex-docs

WHEN `--for DURATION` is given and the wait lasts longer THEN `rex` SHALL
print `gave up after DURATION` and exit with 1.
Without `--for`, `rex wait` SHALL wait without a limit.
WHEN the server shuts down during the wait THEN `rex` SHALL exit with 1.

## Follow

### F-169 Follow events
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex events` SHALL print events as they happen, until the server ends,
then exit with 4.
Without flags, it SHALL print the server's events and the sessions'
events, without the terminals' events.
With `-s SESSION`, it SHALL print that session's events, its terminals'
events too.
With `--all`, it SHALL print every event.
Each line SHALL hold the time, the type, the short ID of the subject, and
the details.
`--json` SHALL print each event as one JSON object on a line.

### F-170 The events
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

| Kind | Types |
|---|---|
| server | `session_created`, `session_destroyed`, `client_joined`, `client_left` |
| session | `window_created`, `window_closed`, `window_label_changed`, `active_window_changed`, `block_created`, `block_closed`, `session_label_changed`, `session_view_changed` |
| terminal | `child_exited`, `bell`, `title_changed`, `pwd_changed`, `process_changed`, `desktop_notification`, `program_status_changed`, `program_status_removed` |

A session event SHALL come as `session_event`, with `session_id` and the
event in `event`.
A terminal event SHALL be a session event `block_event`, with
`block_id`, `name` and `payload`.
Text output SHALL name a terminal event `terminal.NAME`.

### F-171 Program and directory changes
status: proposed · traces: B-4 · source: code

While a client follows a session's events, or all events, the server
SHALL look at each block every second.
WHEN a block's program or directory changed THEN it SHALL send
`process_changed` or `pwd_changed`.

## Report

### F-172 Report a program status
status: proposed · traces: B-4 · source: code, tests

`rex status STATE` SHALL report a program status for the block, with
the effect of an OSC 7501 sequence that its program prints.
`STATE` SHALL be `idle`, `working`, `blocked`, `done`, `error` or `clear`.
`--kind` SHALL be `permission`, `question` or `auth`, for `blocked` only.
`--progress PCT`, from 0 to 100, SHALL count for `working` and `blocked`
only.
`--app`, `--title`, `--msg` (`-m`) and `--id` SHALL give the record's
fields.
WHEN the report is not valid THEN `rex` SHALL exit with 2.

### F-173 Show a block's program status
status: proposed · traces: B-4 · source: code, tests

`rex status` without a state SHALL print the block's status records, one
a line, or `no status`.
A line SHALL hold the record's ID, state, kind, progress, app, title,
message, and its age.
`--json` SHALL print `block_id` and `records`.
