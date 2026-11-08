# rex: sessions, windows, blocks and their layout

Targets and defaults are in `cli.md` (F-110 to F-115).

## Create

### F-130 List sessions
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex ls` and `rex session ls` SHALL print a table of the sessions, in
their order, with the columns `ID`, `LABEL`, `WINDOWS`, `BLOCKS` and
`CREATED`.
`--json` SHALL print `{"sessions": [...]}`.
Each session SHALL have `session_id`, `label`, `created`,
`active_window_id` and `windows`.
Each window SHALL have `window_id`, `label`, `index` from 1, `block_ids`,
`focused_block_id`, `zoomed_block_id` and `layout`.

### F-131 Create a session
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex new [LABEL] [-- COMMAND...]` and `rex session new` SHALL create a
session with one window of one block, after the other sessions.
The block SHALL run the command, else the shell, in the directory `rex`
runs in, unless `--cwd` names another.
`--window LABEL` SHALL label the window.
`rex` SHALL print the session's ID.
`--json` SHALL print `session_id`, `window_id`, `block_id`, `block_ids`,
`label` and `revision`.
WHEN two labels are given THEN `rex` SHALL exit with 2.

### F-132 Options of a new block
status: proposed · traces: B-3 · source: code, rex-docs

`rex new`, `rex run`, `rex split` and `rex window new` SHALL take:

| Flag | Effect |
|---|---|
| `-c`, `--cwd`, `-d`, `--dir DIR` | the directory; `~` is the home directory; made absolute |
| `--keep-open` | keep the block when its command exits (F-139) |
| `--shell SHELL` | run the command through `SHELL`; `none` runs it directly |
| `--interactive` | the shell also reads its interactive startup files |
| `--label TEXT` | the block's label (`run` and `split` only) |

WHEN the directory does not exist THEN the block SHALL start in the home
directory.

### F-133 How a command starts
status: proposed · traces: B-3 · source: code, rex-docs

WHEN a block has no command THEN it SHALL run the user's login shell.
WHEN it has a command THEN the server SHALL run it through the login
shell's environment: the profile's `PATH` applies, its aliases do not.
For `sh`, `bash`, `zsh`, `dash`, `ksh`, `mksh`, `fish`, `ash` and `yash`,
the shell SHALL `exec` the command with its arguments unchanged.
For other shells, and with `--shell none`, the server SHALL start the
command directly.
The command SHALL NOT go through a shell's parsing: pipes need `sh -c`.

### F-134 Run a command in a new window
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex run [flags] -- COMMAND...` SHALL open a window, after the session's
active window, whose block runs the command.
Its directory SHALL be the one `rex` runs in, unless `--cwd` names
another.
WHEN `--split DIRECTION` is given THEN `rex run` SHALL do as `rex split`
instead (F-135).
WHEN there is no command THEN `rex` SHALL exit with 2.

### F-135 Split a block
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex split [flags] [-- COMMAND...]` SHALL put a new block beside the
anchor: `-b`, else the block of F-112.
`--split DIRECTION` SHALL give the side: `right` (the default), `left`,
`above` or `below`.
`--ratio PCT` SHALL give the new block's share of the split; the default
is 50.
The new block SHALL start in the anchor's directory, unless `--cwd` names
another.
The split SHALL end the window's zoom.

### F-136 Focus after a new block
status: proposed · traces: B-4 · source: code, rex-docs, tests

WHEN `rex run`, `rex split` or `rex window new` creates a block THEN the
block SHALL take the focus, and its window SHALL become active.
WHEN `--focus=false` is given THEN the focus and the active window SHALL
stay as they were.

### F-137 What run and split print
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex run` and `rex split` SHALL print the new block's ID.
With `--json`, `rex split` SHALL print `block_id`, `block_ids`,
`split_ids`, `anchor_block_id`, `window_id`, `session_id` and
`revision`.
With `--json`, `rex run` without `--split` SHALL print `session_id`,
`window_id`, `block_id`, `block_ids` and `revision`.

### F-138 Wait for the command
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

WHEN `--wait` is given THEN `rex run` and `rex split` SHALL print the
block's ID, wait for its program, and exit with its exit status (F-105).
`--timeout` SHALL NOT end this wait.

### F-139 When the command exits
status: proposed · traces: B-3 · source: code, rex-docs, tests

WHEN a block's program exits THEN the server SHALL close the block, as
`rex close` does.
WHEN the block was created with `--keep-open` THEN the block SHALL stay.
It SHALL show `[process exited with code N]` and take no more input.

## Close

### F-140 Close blocks
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex close [BLOCK...]` and `rex block close` SHALL close each block named.
Without a block, `rex close` SHALL close the block of F-112.
The block's program SHALL get `SIGHUP`, then `SIGKILL` after 1 second.
The focus SHALL go to the block of the window that had it before.
WHEN a window loses its last block THEN it SHALL close.
WHEN a session loses its last window THEN it SHALL end.

### F-141 End sessions
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex kill SESSION...` and `rex session kill` SHALL end each session named,
with every block in it.
WHEN no session is named THEN `rex` SHALL exit with 2.
`rex` SHALL end the sessions in order, and stop at the first target that
fails.

## Arrange

### F-142 Move the focus
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex focus DIRECTION` SHALL move the focus from the focused block of the
window of F-111.
`left`, `right`, `up` and `down` SHALL go to the nearest block on that
side; `next` and `prev` to the next or previous block in layout order.
At an edge the focus SHALL stay, unless `--wrap` is given.
In a zoomed window, `left`, `right`, `up` and `down` SHALL NOT move the
focus.
The window SHALL become active.

### F-143 Focus a window or block
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex focus TARGET` SHALL make a window active, or give a block the focus.
WHEN the target matches both a window and a block THEN `rex` SHALL exit
with 3, unless it is a full ID.
Focus on a block SHALL end the zoom of another block of its window.

### F-144 Zoom
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex zoom [BLOCK]` and `rex block zoom` SHALL toggle the zoom of a block.
`--on` SHALL zoom it, and `--off` SHALL end its zoom.
WHEN both are given THEN `rex` SHALL exit with 2.
A zoomed block SHALL take the focus.

### F-145 Move a block
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex move [BLOCK] DIRECTION ANCHOR` SHALL take the block out of its window
and put it beside the anchor.
`--ratio PCT`, from 1 to 99, SHALL give its share of the new split.
The moved block SHALL take the focus, and its new window SHALL become
active.
A window left without blocks SHALL close.
WHEN the anchor is the block, or in another session, THEN `rex` SHALL
exit with 2.

### F-146 Swap two blocks
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex swap BLOCK OTHER` SHALL swap the places of two blocks of one session.
WHEN they are in different sessions THEN `rex` SHALL exit with 2.

### F-147 Move a divider
status: proposed · traces: B-3 · source: code

`rex resize [BLOCK] DIRECTION` SHALL move the nearest divider of the block
on that axis, toward the direction.
`--amount PCT` SHALL give how far, as a share of the split; the default
is 5.
A split's ratio SHALL stay between 10 and 90 percent.
WHEN the block has no divider on that axis THEN `rex` SHALL exit with 2.

### F-148 Words for directions
status: proposed · traces: B-3 · source: code

`rex` SHALL take these words, in any case:

| Direction | Words |
|---|---|
| right | `right`, `r`, `beside`, `horizontal` |
| left | `left`, `l` |
| above | `above`, `up`, `top`, `u` |
| below | `below`, `down`, `bottom`, `d`, `vertical` |

WHEN a direction is not in the table THEN `rex` SHALL exit with 2.

## Sessions, windows and blocks

### F-149 Inspect, rename and move sessions
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex session inspect [SESSION]` SHALL print the session and its windows.
`rex session rename [SESSION] LABEL` SHALL set its label, without spaces
at either end.
`rex session move [SESSION]` SHALL take exactly one of `--before S`,
`--after S` or `--index N`.
An index SHALL count from 1; one past the end SHALL move the session
last.
WHEN none or two of these are given THEN `rex` SHALL exit with 2.

### F-150 Create a window
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex window new [LABEL] [-- COMMAND...]` SHALL open a window, after the
active window of the session of F-110 or `-s`.
With a command, its block SHALL start in the directory `rex` runs in.
Without one, it SHALL start in the directory of the focused block of the
active window.
`rex` SHALL print the window's ID.

### F-151 List, inspect and arrange windows
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex window ls` SHALL list the windows of a session, or of every session
with `-A`.
`rex window inspect [WINDOW]` SHALL print the window and its blocks.
`rex window focus [WINDOW|next|prev]` SHALL make the window active.
At the first or last window, `next` and `prev` SHALL change nothing,
unless `--wrap` is given.
`rex window move` SHALL take the flags of `rex session move` (F-149).
`rex window rename [WINDOW] LABEL` SHALL set its label.
`rex window close WINDOW...` SHALL close each window and its blocks.

### F-152 List and inspect blocks
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex block ls` SHALL list the blocks of the session of F-110, of the
window of `-w`, or of every session with `-A`.
Its columns SHALL be `ID`, `LABEL`, `WINDOW`, `PROGRAM`, `STATUS`, `PID`,
`SIZE` and `DIR`.
A block whose program exited SHALL show `exited (N)` as its program.
`rex block inspect [BLOCK]` SHALL print the block's place, process,
program, directory, size, output, and status records.

### F-153 The whole tree
status: proposed · traces: B-3 · source: code

`rex list` SHALL print every session, its windows and their blocks, with
each block's process ID, program, size and directory.
`--json` SHALL print `{"model": ..., "blocks": [...]}`.

### F-154 Rename and focus a block
status: proposed · traces: B-3 · source: code, rex-docs

`rex block rename [BLOCK] LABEL` SHALL set the block's label.
`rex block focus [BLOCK]` SHALL give the block the focus, as F-143 does.

### F-155 Block types
status: proposed · traces: B-5 · source: code, rex-docs

`rex block types` SHALL print one type,
`com.superlogical.terminal.shell`.
`rex block new` SHALL exit with 2: `rex run` and `rex split` create
blocks.

### F-156 Clients
status: proposed · traces: B-3 · source: code, rex-docs

`rex client ls` SHALL list the clients connected to the server.
It SHALL leave out clients of kind `cli`, unless `--all` is given.
`rex client inspect [CLIENT]` SHALL print one client as JSON: the one
named by ID, end of ID or name, else the first app.
WHEN no client matches THEN `rex` SHALL exit with 3.
