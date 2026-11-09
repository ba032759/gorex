# Glossary

The terms of the spec, one meaning each. A word under `avoid:` is not
used in items for that meaning; `spec check` warns where it is.
Differences between Rex, the code and the app are in `FINDINGS.md`.

### GoRex
The product: the app, the server and `rex`, in one executable.

### Rex
Superlogical's terminal for macOS and iOS, with its Linux and FreeBSD
server, which GoRex follows.

### rex command
The command line program of GoRex, written `rex`: the executable run by
the name `rex`, or as `GoRex COMMAND`. ("Rex", capitalized, is
Superlogical's product.)

### app
The GoRex application with a window on the screen of the user's system.
It is a client of kind `app`.
avoid: GUI

### server
The background process that owns the model and runs every block's
program. One server runs for each data directory.
avoid: daemon, session server

### client
A program connected to the server's control socket: the app (kind `app`)
or `rex` (kind `cli`).

### data directory
The directory of the server's socket, lock, log, state, settings, keymap
and wide events: `GOREX_DIR`, else the user's configuration directory
and `GoRex`.
avoid: state dir

### model
What the server keeps of the user's work: the sessions, their windows,
their layouts, labels, focus and zoom, and a revision that rises with
every change.

### session
A group of related work: an ordered list of windows, one of them active.
It keeps running whether or not an app shows it.

### window
An item of a session that holds one layout of blocks. The app shows it
as a tab, as Rex's app does; `rex`, its help and the skill say window.
avoid: tab

### active window
The window of a session that the session shows.

### block
One terminal in a window, with its program: a leaf of a layout. The app
shows it as a pane, as Rex's app does; `rex`, its help and the skill say
block.
avoid: pane

### terminal
The pseudo-terminal of a block, and the screen and scrollback the server
keeps of it.

### layout
The tree of a window: a block, or a split of two layouts.

### split
A node of a layout that divides its room between two layouts, side by
side or one above the other, by a ratio.

### ratio
The share, from 0 to 1, of a split's room that one side has. On the
command line, a percentage.

### anchor
The block that a new or moved block goes beside.

### direction
The side of an anchor, or the way the focus moves: right, left, above
(up) or below (down).

### focus
The one block of a window that takes the user's keys.

### zoom
A block that fills its window, the other blocks hidden.

### label
A name the user gives a session, window or block. It is not unique and
can change.

### ID
The name the server gives a session, window, block, split or client:
its kind, a colon and 12 hexadecimal digits. It never changes and is
never reused.

### short ID
How listings show an ID: `…` and the last six hexadecimal digits.

### target
A command's argument or flag that names a session, window, block or
client: an ID, a label, a position, or the end of an ID.

### context
The session and block `rex` runs in, from `REX_SESSION` and `REX_BLOCK`.

### program
The process in the foreground of a block's terminal; the shell when the
shell is at its prompt.

### shell
The user's login shell, which a block runs when it has no command.

### command
The program and arguments that a block runs instead of the shell, given
after `--`.

### close
To take a block or window out of the model and end its program. A
session that loses its last window ends.
avoid: kill

### keep open
An option of a new block: when its command exits, the block stays and
shows the exit status.

### exit status
The number `rex` exits with, or the number a block's program exited
with.

### event
A message the server sends to the clients that subscribed: a change of
the model, or something a terminal reported.

### wide event
One JSON line in the data directory's `events/` that records a request,
an event or a `rex` command, for later queries.

### program status
What a program reports of its work (the OSC 7501 protocol): idle,
working, blocked, done or error.

### status record
One program status of a block, under an ID path; a block has several.

### action
A named operation of the app, as a key binding runs it: `pane.split`,
`client.tab.goto`.

### keymap
The key bindings in effect: a preset's, then those of `keymap.json`.

### preset
A set of default key bindings: `linux`, `macos`, `darwin` or `none`.

### agent
A coding agent that runs in a block, such as Claude Code, Codex,
OpenCode or Pi.

### skill
The instructions that teach an agent `rex`, in a `SKILL.md` file.

### hook
A command that an agent runs at its own events, which `rex skill setup`
adds to the agent's settings.

### user
The person who uses GoRex.

### owner
The person who decides what GoRex is meant to do.
