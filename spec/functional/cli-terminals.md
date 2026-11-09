# rex: type, read, wait, follow and report

The block these commands act on is that of F-112, or of `-b`, `-s` and
`-w`. What they print, the bytes of a key and the form of an event are
in `technical/cli-terminals.md` (T-160, T-161). The flags each command
takes are in `reference/cli.md`.

## Type

### F-160 Send text
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex send [TEXT...]` and `rex block send` SHALL type the text into the
block's terminal, as the user types it.
WHEN the text is several arguments, `rex` SHALL join them with one space,
those after `--` too.
WHEN no text is given, `rex` SHALL type what it reads from standard
input.
`rex send` SHALL NOT add a newline or press `Enter`.
IF the block's program has exited, THEN `rex` SHALL exit with 1.

### F-161 Press keys
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex send-key KEY...` and `rex block send-key` SHALL press each key, in
order, as one input to the block's terminal.
IF one of the keys is not valid (F-162), THEN `rex` SHALL send none of
them and exit with 2.
IF no key is given, THEN `rex` SHALL exit with 2.
WHEN `--action release` is given, `rex` SHALL send nothing, as a
terminal sends nothing when a key is released.
WHEN `--action` has another value, `rex` SHALL press the keys.

### F-162 How keys are written
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

A key is written as Rex writes it: modifiers and a key name, joined by
`+`. The modifiers, and the names of more than one character, are taken
in any case.

| Part | Written |
|---|---|
| shift | `shift` |
| alt | `alt`, `opt`, `option`, `meta` |
| control | `ctrl`, `control` |
| super | `super`, `cmd`, `command`, `win` |
| a named key | `enter`, `tab`, `escape`, `backspace`, `space`, `up`, `down`, `left`, `right`, `home`, `end`, `insert`, `delete`, `pageup`, `pagedown`, `f1` to `f12` |
| another name of it | `return`, `cr`, `esc`, `bs`, `del`, `ins`, `pgup`, `page_up`, `pgdn`, `page_down` |
| a character | the character, or its name: `plus`, `minus`, `equal`, `comma`, `period`, `slash`, `backslash`, `semicolon`, `quote`, `backquote`, `bracketleft`, `bracketright` |
| a key by its place on a US keyboard | `[KeyA]`, `[Digit1]`, `[ArrowUp]`, `[Slash]`, `[Space]`, `[Enter]` and the other names of the web's `KeyboardEvent.code` |

`ctrl++` SHALL be control and plus.
The server SHALL send the bytes that xterm sends for the key (T-160).
WHEN a key has `shift`, the server SHALL send the character that a US
keyboard types with shift.
IF a modifier or a key name is not in the table, or `ctrl` goes with a
character that has no control character, THEN the key SHALL be not
valid.
WHEN a key has `super`, the server SHALL send the key without it.

### F-163 The bytes of a key
status: superseded · superseded-by: F-162, T-160 · traces: B-3 · source: code, tests

The bytes are a format: T-160. The rule that a key without a control
character is not valid is a rule of F-162.

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
WHEN `--trim` is given, `rex` SHALL drop the blank lines at the end, and
the spaces at the end of each line.
WHEN `--format vt` is given, `rex` SHALL print the escape sequences that
draw the scrollback and the screen, with their styles.
IF `--format` is not `text` or `vt`, as `html`, THEN `rex` SHALL exit
with 2 before it connects to the server.
The output SHALL end with a newline.

### F-165 Keep a block's scrollback
status: proposed · traces: B-2, B-3 · source: code, rex-docs, tests, owner

WHEN a line leaves the top of a block's screen, the server SHALL keep it
in the block's scrollback, up to about 64 MiB of output, as Rex does.
WHEN the scrollback is full, the server SHALL drop its oldest lines.
WHILE a full-screen program shows its own screen, the server SHALL keep
no scrollback of that screen.

## Wait

### F-166 Wait for a block, window or session
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex wait TARGET` SHALL match the target as a block, else as a window,
else as a session (F-113, F-114).
WHEN the target is a block, `rex` SHALL wait for its program to exit,
also in a block kept open, and then exit with its exit status (F-105).
WHEN the target is a window or a session, `rex` SHALL wait for it to
close, and then exit with 0.
WHEN the target is the full ID of a block that closed less than 10
minutes before, `rex` SHALL exit with its program's exit status at once.
IF a block closed and its exit status is not known, THEN `rex` SHALL exit
with 1.
IF `--for DURATION` is given and the wait lasts longer, THEN `rex` SHALL
print `gave up after DURATION` and exit with 1.
IF no target, or more than one, is given, THEN `rex` SHALL exit with 2.

Without `--for`, `rex wait` waits without a limit; `--timeout` does not
end it. With `--json`, it prints the target that ended the wait and its
exit status (T-161).

### F-167 Wait for a block that closed
status: superseded · superseded-by: F-166 · traces: B-3 · source: code, tests

A block that closed is a variant of the same wait, so a rule of F-166.

The server SHALL keep the exit status of a closed block for 10 minutes.
WHEN `rex wait` names a block that closed in that time THEN it SHALL exit
with that status at once.
WHEN the block is gone and its status is not known THEN `rex` SHALL exit
with 1.

### F-168 Give up waiting
status: superseded · superseded-by: F-166 · traces: B-3 · source: code, rex-docs, owner

`--for` is a variant of the same wait, so a rule of F-166. A server that
goes away is F-104's rule for every command.

IF `--for DURATION` is given and the wait lasts longer, THEN `rex` SHALL
print `gave up after DURATION` and exit with 1.
WHEN no `--for` is given, `rex wait` SHALL wait without a limit.
IF the server goes away during the wait, THEN `rex` SHALL exit with 4.

## Follow

### F-169 Follow events
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex events` SHALL print each event as it happens, until the server ends
or the user interrupts it.
WHEN no flag is given, `rex events` SHALL print the server's events and
every session's events, without the terminals' events and the clients'
events.
WHEN `-s SESSION` is given, `rex events` SHALL print that session's
events only, its terminals' events too.
WHEN `--all` is given, `rex events` SHALL print every event.

A line tells when the event happened, its type, the item it concerns,
and its details. `--json` prints each event as the server sends it, on
one line (T-161).

### F-170 The events
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

The server SHALL send an event of this table each time the change it
names happens:

| Kind | Types |
|---|---|
| server | `session_created`, `session_destroyed`, `client_joined`, `client_left` |
| session | `window_created`, `window_closed`, `window_label_changed`, `active_window_changed`, `block_created`, `block_closed`, `session_label_changed`, `session_view_changed` |
| terminal | `child_exited`, `bell`, `title_changed`, `pwd_changed`, `process_changed`, `desktop_notification`, `program_status_changed`, `program_status_removed` |

A session event SHALL name its session, and a terminal event its block
and its session.
`rex events` SHALL write a terminal event as `terminal.` and its type,
as Rex's scripts name it.

How an event is sent, and the fields of each kind, are in T-106 and
T-161.

### F-171 A block's program or directory changes
status: proposed · traces: B-4 · source: code

WHILE a client follows a session's events, or every event, WHEN a
block's program or directory changes, the server SHALL send
`process_changed` or `pwd_changed` within about a second.
WHILE no client follows them, the server SHALL NOT send these two
events.

## Report

### F-172 Report a program status
status: proposed · traces: B-4 · source: code, tests

`rex status STATE` SHALL set a status record of the block, as an OSC
7501 report of its program does (F-460).
`STATE` SHALL be `idle`, `working`, `blocked`, `done`, `error` or
`clear`.
WHEN `STATE` is `clear`, the server SHALL remove the record of `--id`
and the records under it, or, without `--id`, every record of the block.
The server SHALL keep `--kind` for `blocked` only, and `--progress` from
0 to 100 for `working` and `blocked` only, and SHALL ignore them
otherwise.
IF the report is not valid (T-110), THEN `rex` SHALL exit with 2 and
change no record.

`--app`, `--title`, `--msg` and `--id` give the record's fields. `rex
status` is GoRex's own: a hook that has no terminal reports with it
(Finding 20).

### F-173 Show a block's program status
status: proposed · traces: B-4 · source: code, tests

`rex status` without a state SHALL print the block's status records,
one a line, in the order of their IDs.
WHEN the block has no record, `rex status` SHALL print `no status`.
WHEN a record has no app, `rex status` SHALL show the app of the nearest
record above it in the ID path.

A line tells the record's ID, state, kind, progress, app, title,
message, and how long ago it was set (T-161).

### F-460 A program reports its status in its terminal
status: proposed · traces: B-4, B-5 · source: code, rex-docs, tests

WHEN a block's program prints an OSC 7501 report, the server SHALL set
the status record that the report names, as Rex's protocol says (T-110).
IF the report is not valid, THEN the server SHALL ignore it and change
no record.
WHEN the program prints the query `OSC 7501 ; ?`, the server SHALL
answer in the terminal that it supports the protocol.
WHEN a block has 256 records and a report adds one, the server SHALL
remove the record set least recently.

### F-461 A program status ends
status: proposed · traces: B-4 · source: code, rex-docs, tests

WHEN a block's program exits, or its shell shows a prompt, the server
SHALL remove the block's `working` and `blocked` records.
WHEN the shell marks the start of the user's next command (OSC 133), the
server SHALL remove the block's `done`, `error` and `idle` records.
WHEN the program resets the terminal fully (`RIS`), the server SHALL
remove every record of the block.

The server sees a prompt by its OSC 133 mark, or, for a shell that marks
none, by the shell's being at its prompt, which it checks every second.
Rex's protocol keeps `done` and `error` until they are replaced or
cleared, and lets the terminal decide when to stop showing them; GoRex
ends them at the next command.
