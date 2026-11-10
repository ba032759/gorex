# rex: sessions, windows, blocks and their layout

Targets and defaults are in `cli.md` (F-109 to F-115): a command that
names no target acts in its context. What the commands print, and how
the server starts and ends a program, are in `technical/cli-layout.md`
(T-124 to T-126). The flags each command takes are in
`reference/cli.md`.

## Create

### F-130 List sessions, windows or blocks

status: accepted · traces: B-3, B-4 · source: code, rex-docs, tests

`rex ls` and `rex session ls` SHALL list the sessions, in their order.
`rex window ls` SHALL list the windows of the session of F-110, and `rex
block ls` the blocks of that session, in their order.
WHERE `-A` is given, `rex window ls` and `rex block ls` SHALL list those
of every session.
`rex list` SHALL list every session, with its windows and their blocks.
WHEN a block's program exited, `rex block ls` SHALL show `exited (N)` as
its program.

A listing tells where each item is, what it holds, and, for a block,
what runs in it and in which directory. `--json` also tells each
window's layout, focus and zoom. T-124 lists the columns and fields.

### F-131 Create a session

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex new [LABEL]` and `rex session new` SHALL create a session, after the
others, with one window of one block.
WHEN a command follows `--`, the block SHALL run it instead of the shell.
`rex new` SHALL print the new session's ID.
IF two labels are given, THEN `rex` SHALL exit with 2.

### F-132 A new block

status: accepted · traces: B-3, B-4 · source: code, rex-docs, tests, owner

`rex new`, `rex run`, `rex split` and `rex window new` SHALL take these
options for the block they create:

| Option                           | Effect                                                                                                                                |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `-c`, `--cwd`, `-d`, `--dir DIR` | the block's directory; `~` is the user's home directory, `~NAME` that user's; a relative path starts from the directory `rex` runs in |
| `--keep-open`                    | the block stays when its command exits (F-139)                                                                                        |
| `--shell SHELL`                  | the shell whose environment runs the command; `none` starts it directly (F-133)                                                       |
| `--interactive`                  | the shell also reads its interactive startup files (F-133)                                                                            |
| `--label TEXT`                   | the block's label; `rex run` and `rex split` only                                                                                     |
| `--focus=false`                  | the focus and the active window stay as they were; not `rex new`                                                                      |

WHEN `rex split` or `rex run --split` creates a block without `--cwd`,
the block SHALL start in the directory of the block it goes beside.
WHEN `rex new`, `rex run` or `rex window new` creates a block without
`--cwd`, the block SHALL start in the directory `rex` runs in.

WHEN `rex run`, `rex split` or `rex window new` creates a block without
`--focus=false`, the block SHALL take the focus and its window SHALL
become active.
IF the directory does not exist, THEN the block SHALL start in the home
directory.

### F-133 Start a block's command

status: accepted · traces: B-3 · source: code, rex-docs

WHEN a block has no command, the server SHALL run the user's login shell
in it.
WHEN a block has a command, the server SHALL run it in the environment
of the login shell: the profile's `PATH` applies, its aliases do not.
WHERE `--shell none` is given or the shell is not one that T-125 names,
the server SHALL start the command directly, without the profile.
The server SHALL pass the command's arguments to the program unchanged,
without a shell's parsing.
IF the server starts the command directly and does not find it, THEN
`rex` SHALL exit with 1 and create no block.

### F-134 Run a command

status: accepted · traces: B-3, B-4 · source: code, rex-docs, tests

`rex run -- COMMAND...` SHALL run the command in a new block, in a new
window that it places as `rex window new` does (F-150).
WHEN `--split DIRECTION` is given, `rex run` SHALL put the block beside
the anchor instead, as `rex split` does (F-135).
`rex run` SHALL print the new block's ID, also when it opens a window.
WHEN `--wait` is given, `rex run` or `rex split` SHALL then wait for the
block's program to exit, and exit with its exit status (F-105).
WHILE `rex` waits for the program, `--timeout` SHALL NOT end the wait.
IF no command follows `--`, THEN `rex` SHALL exit with 2.

### F-135 Split a block

status: accepted · traces: B-3, B-4 · source: code, rex-docs, tests, owner

`rex split [-- COMMAND...]` SHALL put a new block beside the anchor: the
block of `-b`, else the block of F-112.
The split SHALL end the zoom of the window.
IF `--ratio` is not from 1 to 99, THEN `rex` SHALL exit with 2, and say the range.
The server SHALL hold the ratio from 5 to 95 percent (T-107).

### F-136 Focus after a new block

status: superseded · superseded-by: F-132 · traces: B-4 · source: code, rex-docs, tests

WHEN `rex run`, `rex split` or `rex window new` creates a block THEN the
block SHALL take the focus, and its window SHALL become active.
WHEN `--focus=false` is given THEN the focus and the active window SHALL
stay as they were.

### F-137 What run and split print

status: superseded · superseded-by: F-134, F-135 · traces: B-3 · source: code, rex-docs, tests

The ID they print is now a rule of F-134 and F-135; their JSON is T-124.

`rex run` and `rex split` SHALL print the new block's ID.
With `--json`, `rex split` SHALL print `block_id`, `block_ids`,
`split_ids`, `anchor_block_id`, `window_id`, `session_id` and
`revision`.
With `--json`, `rex run` without `--split` SHALL print `session_id`,
`window_id`, `block_id`, `block_ids` and `revision`.

### F-138 Run a command and wait for it

status: superseded · superseded-by: F-134 · traces: B-3, B-4 · source: code, rex-docs, tests

`--wait` is a variant of `rex run`, so it is a rule of F-134.

WHEN `--wait` is given, `rex run` and `rex split` SHALL first print what
they print without it.
They SHALL then wait for the block's program to exit, and exit with its
exit status (F-105).
`--timeout` SHALL NOT end this wait.

### F-139 A block's program exits

status: accepted · traces: B-3 · source: code, rex-docs, tests

WHEN a block's program exits, the server SHALL close the block, as `rex
close` does (F-140).
The server SHALL keep the program's exit status for `rex wait` (F-166).
WHERE the block was created with `--keep-open`, the block SHALL stay and
show `[process exited with code N]`.
WHILE such a block stays, it SHALL take no more input.

## Close

### F-140 Close blocks and windows, end sessions

status: accepted · traces: B-3 · source: code, rex-docs, tests, owner

`rex close [BLOCK...]` and `rex block close` SHALL close each block
named.
`rex window close WINDOW...` SHALL close each window named, with every
block in it.
`rex kill SESSION...` and `rex session kill` SHALL end each session
named, with every block in it.
WHEN a window loses its last block, the server SHALL close the window.
WHEN a session loses its last window, the server SHALL end the session.
WHEN the focused block closes, the focus SHALL go to the block of its
window that had it before.
WHEN the active window closes, the window before it SHALL become active,
else the first.
IF `rex kill` or `rex window close` names nothing, THEN `rex` SHALL exit
with 2.
`rex` SHALL resolve every target before it closes anything.
IF one of the targets matches nothing, or more than one item, THEN `rex`
SHALL change nothing and exit with 3.

T-125 says how the server ends a block's program.

### F-141 End sessions

status: superseded · superseded-by: F-140 · traces: B-3 · source: code, rex-docs, tests

Closing a block, a window or a session is one event, so one item.

`rex kill SESSION...` and `rex session kill` SHALL end each session
named, in turn, with every block in it.
IF no session is named, THEN `rex` SHALL exit with 2.
IF a session matches nothing, THEN `rex` SHALL exit with 3, and the
sessions before it SHALL stay ended.

### F-192 Close windows

status: superseded · superseded-by: F-140 · traces: B-3 · source: code, rex-docs

Closing a block, a window or a session is one event, so one item.

`rex window close WINDOW...` SHALL close each window named, in turn, with
every block in it.
WHEN the active window closes, the window before it SHALL become active,
else the first.
WHEN a session loses its last window, the server SHALL end the session.
IF no window is named, THEN `rex` SHALL exit with 2.
IF a window matches nothing, THEN `rex` SHALL exit with 3, and the
windows before it SHALL stay closed.

## Arrange

### F-142 Move the focus by direction or order

status: accepted · traces: B-3, B-4 · source: code, rex-docs, tests, owner

`rex focus DIRECTION` SHALL move the focus from the block of F-112, and
make its window active.
`left`, `right`, `up` and `down`, in any case, SHALL move it to the
nearest block on that side.
`next` and `prev` SHALL move it to the next or previous block in layout
order.
`rex window focus next` and `prev`, in any case, SHALL make the window after or before
the window of F-111 active.
WHEN the focus or the window is at the edge or the end, it SHALL stay,
unless `--wrap` is given.
WHILE the window is zoomed, `rex focus DIRECTION` SHALL move the focus
as in the window without the zoom.

### F-143 Focus a window or a block

status: accepted · traces: B-3 · source: code, rex-docs, tests, owner

`rex focus TARGET` SHALL make a window active, or give a block the focus
and make its window active.
`rex window focus WINDOW` SHALL make the window active.
`rex block focus BLOCK` SHALL give the block the focus and make its
window active.
WHEN a block takes the focus, by any command or in the app, the server
SHALL end the zoom of another block of its window.
IF the target of `rex focus` matches both a window and a block and is
not a full ID, THEN `rex` SHALL exit with 3 and change nothing.

### F-144 Zoom a block

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex zoom [BLOCK]` and `rex block zoom` SHALL zoom the block, or end its
zoom when it is zoomed.
`--on` SHALL only zoom it, and `--off` SHALL only end its zoom.
A zoomed block SHALL take the focus.
The zoom SHALL NOT change the layout.
IF both `--on` and `--off` are given, THEN `rex` SHALL exit with 2.

### F-145 Move a block

status: accepted · traces: B-3 · source: code, rex-docs, tests, owner

`rex move [BLOCK] DIRECTION ANCHOR` SHALL take the block out of its
window and put it beside the anchor, in the anchor's window.
The moved block SHALL take the focus, and its new window SHALL become
active, without zoom.
WHEN the block leaves a window empty, the server SHALL close that window.
IF the anchor is the block or in another session, THEN `rex` SHALL exit
with 2.
IF `--ratio` is not from 1 to 99, THEN `rex` SHALL exit with 2.
The server SHALL hold the ratio from 5 to 95 percent (T-107).

### F-146 Swap two blocks

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex swap BLOCK OTHER` and `rex block swap` SHALL swap the places of two
blocks of one session.
WHEN the two are in different windows, each window SHALL give the focus
to the block that takes the place of its focused block.
IF the two are in different sessions, THEN `rex` SHALL exit with 2.

### F-147 Resize a block

status: accepted · traces: B-3 · source: code, owner

`rex resize [BLOCK] DIRECTION` SHALL move the nearest split around the
block on that axis toward the direction.
The ratio SHALL stay from 5 to 95 percent (T-107).
IF the block is in no split on that axis, THEN `rex` SHALL exit with 2.
IF `--amount` is not a positive percentage, THEN `rex` SHALL exit with 2.

### F-148 Words for directions

status: accepted · traces: B-3 · source: code

`rex split`, `rex run --split`, `rex move` and `rex resize` SHALL take
these words, in any case:

| Direction | Words                                      |
| --------- | ------------------------------------------ |
| right     | `right`, `r`, `beside`, `horizontal`       |
| left      | `left`, `l`                                |
| above     | `above`, `up`, `top`, `u`                  |
| below     | `below`, `down`, `bottom`, `d`, `vertical` |

IF a direction is not in the table, THEN `rex` SHALL exit with 2.

`rex focus` takes `left`, `right`, `up` and `down` only (F-142).

## Sessions, windows and blocks

### F-149 Reorder sessions and windows

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex session move [SESSION]` SHALL move a session in the order of
sessions, and `rex window move [WINDOW]` a window in its session.
WHEN the index is past the end, `rex` SHALL move the item last.
WHEN the item is in its new place already, `rex` SHALL change nothing and
exit with 0.
IF none or two of `--before`, `--after` and `--index` are given, THEN
`rex` SHALL exit with 2.
IF the index is not a number from 1, THEN `rex` SHALL exit with 2.

### F-150 Create a window

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex window new [LABEL] [-- COMMAND...]` and `rex new-tab` SHALL open a
window of one block after the active window of the session of F-110.
`rex window new` SHALL print the new window's ID.

### F-151 List windows

status: superseded · superseded-by: F-130 · traces: B-3 · source: code, rex-docs

Listing sessions, windows or blocks is one event, so one item.

`rex window ls` SHALL list the windows of the session of F-110, or of
every session with `-A`, in their order.
Each line SHALL tell the window's position, label, session, number of
blocks and focused block.

### F-191 Change the active window

status: superseded · superseded-by: F-142, F-143 · traces: B-3 · source: code, rex-docs, tests

`rex window focus next` and `prev` are F-142; `rex window focus WINDOW`
is F-143.

`rex window focus WINDOW` SHALL make the window active.
Without a window, it SHALL make the window of F-111 active.
`next` and `prev` SHALL make the next or previous window of the session
active.
WHEN the window is the first or the last, `next` and `prev` SHALL change
nothing, unless `--wrap` is given.

### F-152 List blocks

status: superseded · superseded-by: F-130 · traces: B-3, B-4 · source: code, rex-docs, tests

Listing sessions, windows or blocks is one event, so one item.

`rex block ls` SHALL list the blocks of the session of F-110, of the
window of `-w`, or of every session with `-A`.
Each line SHALL tell the block's label, window, program, program status,
process ID, size and directory.
A block whose program exited SHALL show `exited (N)` as its program.

### F-153 Show the whole tree

status: superseded · superseded-by: F-130 · traces: B-3 · source: code

Listing sessions, windows or blocks is one event, so one item.

`rex list` SHALL print every session, its windows and their blocks, with
each block's process ID, program, size and directory.

### F-154 Rename a session, window or block

status: accepted · traces: B-3 · source: code, rex-docs, tests

`rex session rename [SESSION] LABEL`, `rex window rename [WINDOW] LABEL`
and `rex block rename [BLOCK] LABEL` SHALL set the label.
The server SHALL drop the spaces at either end of the label.
IF no label or more than two arguments are given, THEN `rex` SHALL exit
with 2.

### F-190 Inspect a session, window or block

status: accepted · traces: B-3, B-4 · source: code, rex-docs

`rex session inspect`, `rex window inspect` and `rex block inspect` SHALL
print one item, with what its listing tells and more (T-126).
A block SHALL also show its exit status and its status records.

### F-155 Block types

status: accepted · traces: B-5 · source: code, rex-docs

`rex block types` SHALL print one type, `com.superlogical.terminal.shell`.
IF `rex block new` is asked for a block, THEN `rex` SHALL exit with 2:
`rex run` and `rex split` create blocks.

### F-156 List clients

status: accepted · traces: B-3 · source: code, rex-docs, tests, owner

`rex client ls` SHALL list the clients connected to the server, each with
its kind, name, age and the session it shows.
The table SHALL leave out clients of kind `cli`, unless `--all` is given.
WHEN `--json` is given, `rex` SHALL print every client.
WHEN `-q` is given, `rex` SHALL print the IDs of the clients alone, one a
line, as `rex ls -q` does.

### F-193 Inspect a client

status: accepted · traces: B-3 · source: code, rex-docs

`rex client inspect [CLIENT]` SHALL print an app and the actions it
offers; `--json` SHALL print them as JSON.
It SHALL find the app as `rex do -C` does (F-113): `CLIENT`, else `-C`
or `REX_CLIENT`, else the app when exactly one is connected.
IF no app is connected, THEN `rex` SHALL exit with 1, as for an action
of the app (F-175).
IF no app matches, or several do and none is named, THEN `rex` SHALL
exit with 3.
