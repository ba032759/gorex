# Business: who GoRex is for, and why

### B-1 Rex's way of working, on Linux
status: proposed · source: code, rex-docs
rationale: unknown

The user works in terminals and likes Rex's model: sessions of windows of
blocks, kept by a server. Rex runs as an app on macOS and iOS only; its
Linux and FreeBSD builds are servers without an app. GoRex gives the user
that model with an app on Linux, and also on macOS and Windows.

It fails if the user who knows Rex has to learn GoRex again. Why GoRex
has its own server, and not Rex's Linux server, is in D-11.

### B-2 Work outlives the app
status: proposed · source: rex-docs, code

The user keeps long work in terminals: editors, servers, builds, agents.
A session keeps running whether or not an app shows it. When the user
quits the app and opens it again, every session, window and block is back,
and the programs in them still run.

When the server itself ends, as at a reboot, the user gets the layout back
with shells in the same directories. It fails if quitting the app loses
work.

### B-3 Drive terminals from scripts
status: proposed · source: rex-docs, code

The user automates the work of the terminal from ordinary shell scripts.
Scripts create sessions and blocks, type into them, read their screens,
wait for their programs, and react to events. A script needs stable IDs, output
that a program reads (JSON), and exit statuses that tell what went wrong.

It fails if a script must parse text made for people, or cannot tell a
missing target from a failed command.

### B-4 Agents work beside the user
status: proposed · source: rex-docs, code

Coding agents, such as Claude Code and Codex, run in the user's terminals.
They need to open terminals beside themselves, run commands there, wait,
and read the screens. They need to know where they run without searching
for it. The user needs to see what an agent does, and when it waits for
the user.

It fails if agents spend time and tokens to find their place, or take the
focus from the user.

### B-5 What works with Rex works with GoRex
status: proposed · source: code, rex-docs
rationale: unknown

Scripts, habits and agent skills written for Rex's `rex` command work on
GoRex without change: the same commands, flags, targets, output and exit
statuses. Where GoRex does less than Rex, it says so clearly.

The repository states this as a goal, not its reason. A likely reason is
that agents trained on Rex's documentation drive GoRex too; the owner has
not said so.

### B-6 One machine, one user
status: proposed · source: code

GoRex runs on the machine of the user who uses it. Its server listens on
a local socket that only that user reads. It has no remote hosts, no
shared or login servers, and no network access.

### B-7 A small, native app on three systems
status: proposed · source: code

GoRex runs on Linux, macOS and Windows (10 1809 or later). It is one
executable of about 15 MB, with a native interface and no web view.
The app, the server and the `rex` command are that one executable.

### B-8 Observe it without exposing the user
status: proposed · source: code, rex-docs

The owner and agents need to see what GoRex does in real use, to find
faults and check features. The user types secrets into terminals. What
GoRex records of its use stays on the machine and holds no text that the
user typed or that programs reported.

### B-9 Built and kept by agents for one owner
status: proposed · source: code
rationale: unknown

GoRex is a project of one owner, built mostly by coding agents, without a
specification at first. The spec holds the intent, so that agents change
GoRex without breaking what the owner wanted.
