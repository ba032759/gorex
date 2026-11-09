# rex: sessions, windows, blocks and their layout

Targets and defaults are in `cli.md` (F-110 to F-115). What the commands
print as tables and JSON, and how the server starts a command, are in
`technical/cli-layout.md` (T-124, T-125).

## Create

### F-130 List sessions
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex ls` and `rex session ls` SHALL list the sessions in their order.
Each line SHALL tell the session's label, its numbers of windows and
blocks, and how long ago it was created.
WHEN `--json` is given, `rex` SHALL also print each session's windows,
with their blocks, focus, zoom and layout.

### F-131 Create a session
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-135, F-150

`rex new [LABEL]` and `rex session new` SHALL create a session, after the
others, with one window of one block.
The block SHALL start in the directory that `rex` runs in, unless
`--cwd` names another.
WHEN a command follows `--`, the block SHALL run it instead of the shell.
`--window LABEL` SHALL label the window.
`rex new` SHALL print the new session's ID.
IF two labels are given, THEN `rex` SHALL exit with 2.

### F-132 The options of a new block
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex new`, `rex run`, `rex split` and `rex window new` SHALL take these
options for the block they create:

| Option | Effect |
|---|---|
| `-c`, `--cwd`, `-d`, `--dir DIR` | the block's directory; `~` is the home directory; a relative path starts from the directory `rex` runs in |
| `--keep-open` | the block stays when its command exits (F-139) |
| `--shell SHELL` | the shell whose environment runs the command; `none` starts it directly (F-133) |
| `--interactive` | the shell also reads its interactive startup files (F-133) |
| `--label TEXT` | the block's label; `rex run` and `rex split` only |
| `--focus=false` | the focus and the active window stay as they were; not `rex new` |

WHEN `rex run` or `rex split` or `rex window new` creates a block without
`--focus=false`, the block SHALL take the focus and its window SHALL
become active.
IF the directory does not exist, THEN the block SHALL start in the home
directory.

### F-133 Start a block's command
status: proposed · traces: B-3 · source: code, rex-docs

WHEN a block has no command, the server SHALL run the user's login shell
in it.
WHEN a block has a command, the server SHALL run it in the environment
of the login shell: the profile's `PATH` applies, its aliases do not.
WHERE `--interactive` is given, the shell SHALL also read its interactive
startup files.
WHERE `--shell none` is given or the shell is not one that T-125 names,
the server SHALL start the command directly, without the profile.
The server SHALL pass the command's arguments to the program unchanged:
a pipe needs `sh -c`.
IF the server starts the command directly and does not find it, THEN
`rex` SHALL exit with 1 and create no block.

### F-134 Run a command in a new window
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests · conflicts: F-135

`rex run -- COMMAND...` SHALL open a window after the active window of
the session of F-110, with one block that runs the command.
The block SHALL start in the directory that `rex` runs in, unless
`--cwd` names another.
WHEN `--split DIRECTION` is given, `rex run` SHALL put the block beside
the anchor instead, as `rex split` does (F-135), in the same directory.
`rex run` SHALL print the new block's ID.
IF no command follows `--`, THEN `rex` SHALL exit with 2.

### F-135 Split a block
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests · conflicts: F-131, F-134, F-145, F-150

`rex split [-- COMMAND...]` SHALL put a new block beside the anchor: the
block of `-b`, else the block of F-112.
`--split DIRECTION` SHALL give the side, in the words of F-148; the
default is right.
`--ratio PCT` SHALL give the new block's share of the split; the default
is 50.
The new block SHALL start in the anchor's directory, unless `--cwd`
names another.
The split SHALL end the zoom of the window.
`rex split` SHALL print the new block's ID.
IF `--ratio` is not above 0 and below 100, THEN `rex` SHALL exit with 2.

### F-136 Focus after a new block
status: superseded · superseded-by: F-132 · traces: B-4 · source: code, rex-docs, tests

WHEN `rex run`, `rex split` or `rex window new` creates a block THEN the
block SHALL take the focus, and its window SHALL become active.
WHEN `--focus=false` is given THEN the focus and the active window SHALL
stay as they were.

### F-137 What run and split print
status: superseded · superseded-by: F-135 · traces: B-3 · source: code, rex-docs, tests

The ID they print is now a rule of F-134 and F-135; their JSON is T-124.

`rex run` and `rex split` SHALL print the new block's ID.
With `--json`, `rex split` SHALL print `block_id`, `block_ids`,
`split_ids`, `anchor_block_id`, `window_id`, `session_id` and
`revision`.
With `--json`, `rex run` without `--split` SHALL print `session_id`,
`window_id`, `block_id`, `block_ids` and `revision`.

### F-138 Run a command and wait for it
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

WHEN `--wait` is given, `rex run` and `rex split` SHALL first print what
they print without it.
They SHALL then wait for the block's program to exit, and exit with its
exit status (F-105).
`--timeout` SHALL NOT end this wait.

### F-139 A block's program exits
status: proposed · traces: B-3 · source: code, rex-docs, tests

WHEN a block's program exits, the server SHALL close the block, as `rex
close` does (F-140).
The server SHALL keep the program's exit status for `rex wait` (F-167).
WHERE the block was created with `--keep-open`, the block SHALL stay and
show `[process exited with code N]`.
WHILE such a block stays, it SHALL take no more input.

## Close

### F-140 Close blocks
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-113

`rex close [BLOCK...]` and `rex block close` SHALL close each block named,
in turn.
Without a block, they SHALL close the block of F-112.
The block's program SHALL get `SIGHUP`, then `SIGKILL` after 1 second.
The focus SHALL go to the block of the window that had it before.
WHEN a window loses its last block, the server SHALL close the window, as
`rex window close` does (F-192).
IF a block matches nothing, THEN `rex` SHALL exit with 3, and the blocks
before it SHALL stay closed.

### F-141 End sessions
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-113

`rex kill SESSION...` and `rex session kill` SHALL end each session
named, in turn, with every block in it.
IF no session is named, THEN `rex` SHALL exit with 2.
IF a session matches nothing, THEN `rex` SHALL exit with 3, and the
sessions before it SHALL stay ended.

### F-192 Close windows
status: proposed · traces: B-3 · source: code, rex-docs · conflicts: F-113

`rex window close WINDOW...` SHALL close each window named, in turn, with
every block in it.
WHEN the active window closes, the window before it SHALL become active,
else the first.
WHEN a session loses its last window, the server SHALL end the session.
IF no window is named, THEN `rex` SHALL exit with 2.
IF a window matches nothing, THEN `rex` SHALL exit with 3, and the
windows before it SHALL stay closed.

## Arrange

### F-142 Move the focus to a neighbour
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests · conflicts: F-112, F-143

`rex focus DIRECTION` SHALL move the focus from the focused block of the
window of F-111, and make that window active.
`left`, `right`, `up` and `down`, in any case, SHALL move it to the
nearest block on that side.
`next` and `prev` SHALL move it to the next or previous block in layout
order.
WHEN the focus is at the edge or the end, it SHALL stay, unless `--wrap`
is given.
WHILE the window is zoomed, `rex focus` SHALL NOT move the focus to a
side.

### F-143 Focus a window or a block
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-142

`rex focus TARGET` SHALL make a window active, or give a block the focus
and make its window active.
`rex block focus [BLOCK]` SHALL give a block the focus: the block of
F-112 without one.
WHEN a block takes the focus, the server SHALL end the zoom of another
block of its window.
IF the target matches both a window and a block and is not a full ID,
THEN `rex` SHALL exit with 3 and change nothing.

### F-144 Zoom a block
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex zoom [BLOCK]` and `rex block zoom` SHALL zoom the block, or end its
zoom: the block of F-112 without one.
`--on` SHALL zoom it, and `--off` SHALL end its zoom.
A zoomed block SHALL take the focus.
The zoom SHALL NOT change the layout.
IF both `--on` and `--off` are given, THEN `rex` SHALL exit with 2.

### F-145 Move a block
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-135

`rex move [BLOCK] DIRECTION ANCHOR` SHALL take the block out of its
window and put it beside the anchor, in the anchor's window.
`--ratio PCT` SHALL give the block's share of the new split; the default
is 50.
The moved block SHALL take the focus, and its new window SHALL become
active, without zoom.
WHEN the block leaves a window empty, the server SHALL close that window.
IF the anchor is the block or in another session, THEN `rex` SHALL exit
with 2.
IF `--ratio` is not from 1 to 99, THEN `rex` SHALL exit with 2.

### F-146 Swap two blocks
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex swap BLOCK OTHER` and `rex block swap` SHALL swap the places of two
blocks of one session.
WHEN the two are in different windows, each window SHALL give the focus
to the block that takes the place of its focused block.
IF the two are in different sessions, THEN `rex` SHALL exit with 2.

### F-147 Resize a block
status: proposed · traces: B-3 · source: code

`rex resize [BLOCK] DIRECTION` SHALL change the ratio of the nearest split
around the block on that axis, toward the direction.
`--amount PCT` SHALL give the change, as a share of the split; the
default is 5.
The ratio SHALL stay from 10 to 90 percent.
IF the block is in no split on that axis, THEN `rex` SHALL exit with 2.

### F-148 Words for directions
status: proposed · traces: B-3 · source: code

`rex split`, `rex run --split`, `rex move` and `rex resize` SHALL take
these words, in any case:

| Direction | Words |
|---|---|
| right | `right`, `r`, `beside`, `horizontal` |
| left | `left`, `l` |
| above | `above`, `up`, `top`, `u` |
| below | `below`, `down`, `bottom`, `d`, `vertical` |

IF a direction is not in the table, THEN `rex` SHALL exit with 2.

## Sessions, windows and blocks

### F-149 Reorder sessions and windows
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex session move [SESSION]` SHALL move a session in the order of
sessions, and `rex window move [WINDOW]` a window in its session.
`--before X`, `--after X` or `--index N` SHALL give the new place; an
index counts from 1.
WHEN the index is past the end, `rex` SHALL move the item last.
WHEN the item is in its new place already, `rex` SHALL change nothing and
exit with 0.
IF none or two of the three flags are given, THEN `rex` SHALL exit with 2.

### F-150 Create a window
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-131, F-135

`rex window new [LABEL] [-- COMMAND...]` SHALL open a window, with that
label, after the active window of the session of F-110 or `-s`.
WHEN a command is given, its block SHALL start in the directory that
`rex` runs in.
WHEN no command is given, its block SHALL start in the directory of the
focused block of the active window.
`--cwd` SHALL name another directory in both cases.
`rex window new` SHALL print the new window's ID.

### F-151 List windows
status: proposed · traces: B-3 · source: code, rex-docs

`rex window ls` SHALL list the windows of the session of F-110, or of
every session with `-A`, in their order.
Each line SHALL tell the window's position, label, session, number of
blocks and focused block.

### F-191 Change the active window
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex window focus WINDOW` SHALL make the window active.
Without a window, it SHALL make the window of F-111 active.
`next` and `prev` SHALL make the next or previous window of the session
active.
WHEN the window is the first or the last, `next` and `prev` SHALL change
nothing, unless `--wrap` is given.

### F-152 List blocks
status: proposed · traces: B-3, B-4 · source: code, rex-docs, tests

`rex block ls` SHALL list the blocks of the session of F-110, of the
window of `-w`, or of every session with `-A`.
Each line SHALL tell the block's label, window, program, program status,
process ID, size and directory.
A block whose program exited SHALL show `exited (N)` as its program.

### F-153 Show the whole tree
status: proposed · traces: B-3 · source: code

`rex list` SHALL print every session, its windows and their blocks, with
each block's process ID, program, size and directory.

### F-154 Rename a session, window or block
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex session rename [SESSION] LABEL`, `rex window rename [WINDOW] LABEL`
and `rex block rename [BLOCK] LABEL` SHALL set the label.
Without a target, they SHALL rename the session of F-110, the window of
F-111 or the block of F-112.
The server SHALL drop the spaces at either end of the label.
IF no label or more than two arguments are given, THEN `rex` SHALL exit
with 2.

### F-190 Inspect a session, window or block
status: proposed · traces: B-3, B-4 · source: code, rex-docs

`rex session inspect`, `rex window inspect` and `rex block inspect` SHALL
print one item: the target, else that of F-110, F-111 or F-112.
A session SHALL show its ID, label, creation time and windows, the active
one marked.
A window SHALL show its ID, label, session, position, focused and zoomed
block, and blocks.
A block SHALL show its place, focus, process, program and arguments,
directory, size, output, exit status and status records.

### F-155 Block types
status: proposed · traces: B-5 · source: code, rex-docs

`rex block types` SHALL print one type, `com.superlogical.terminal.shell`.
IF `rex block new` is asked for a block, THEN `rex` SHALL exit with 2:
`rex run` and `rex split` create blocks.

### F-156 List clients
status: proposed · traces: B-3 · source: code, rex-docs

`rex client ls` SHALL list the clients connected to the server, each with
its kind, name, age and the session it shows.
The table SHALL leave out clients of kind `cli`, unless `--all` is given.
WHEN `--json` is given, `rex` SHALL print every client.

### F-193 Inspect a client
status: proposed · traces: B-3 · source: code, rex-docs · conflicts: F-113

`rex client inspect CLIENT` SHALL print, as JSON, the client whose full
ID, end of ID or name is `CLIENT`.
Without a client, it SHALL print the first client of kind `app`.
IF no client matches, THEN `rex` SHALL exit with 3.
