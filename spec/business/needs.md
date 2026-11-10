# Business: who GoRex is for, and why

GoRex is a project of one owner, built mostly by coding agents, at first
without a specification. The spec holds the owner's intent, so that agents
change GoRex without breaking what the owner wanted.

### B-1 Rex's way of working, on Linux

status: accepted · source: rex-docs, code, owner

**Who.** A user who works in terminals and knows Rex.
**Needs.** Rex's way of working on Linux, with an app: sessions of
windows of blocks, kept by a server.
**Why.** Rex has an app on macOS and iOS only. Its Linux server has no
app (D-11).
**Fails if.** A user who knows Rex has to learn GoRex again, or needs a
Mac to see the sessions of a Linux machine.

### B-2 Work outlives the app

status: accepted · source: rex-docs, code

**Who.** A user who keeps long work in terminals: editors, servers,
builds, agents.
**Needs.** Sessions that run on whether or not an app shows them. After
a reboot, the same layout, with shells in the same directories.
**Why.** Long work must not depend on one window staying open.
**Fails if.** Quitting the app stops a program, or a reboot loses the
layout.

### B-3 Scripts drive terminals

status: accepted · source: rex-docs, code

**Who.** A user who automates the work of the terminal.
**Needs.** To create sessions and blocks from a shell script, type into
them, read their screens, wait for their programs and react to events.
**Why.** Repeated work then runs without the user at the keyboard.
**Fails if.** A script must read text made for people, or cannot tell a
missing target from a failed command.

### B-4 Agents work beside the user

status: accepted · source: rex-docs, code

**Who.** A coding agent that runs in the user's terminal, such as Claude
Code or Codex, and the user who works with it.
**Needs.** The agent: to open blocks beside itself, run commands there,
wait for them and read their screens, and to know where it runs. The
user: to see what an agent does, and when it waits for the user.
**Why.** The agent does more of the work, and the user keeps control of
it.
**Fails if.** An agent spends turns to find its place, takes the focus
from the user, or waits for the user without the user seeing it.

### B-5 What works with Rex works with GoRex

status: accepted · source: rex-docs, code

**Who.** A user, a script or an agent skill made for Rex's `rex` command.
**Needs.** To work on GoRex without change.
**Why.** Unknown.
**Fails if.** A script made for Rex fails on GoRex where GoRex has the
feature, or GoRex does less than Rex and does not say so.

### B-6 Only the user reaches their terminals

status: accepted · source: code, rex-docs

**Who.** The user, who types secrets into terminals and runs programs
with their own rights.
**Needs.** Only the user, and the clients the user allows, read their
screens or drive their terminals.
**Why.** A terminal holds secrets and acts as the user.
**Fails if.** Another user of the machine, or another machine, reads a
screen or types into a block without the user's permission.

### B-7 Linux only

status: accepted · kind: constraint · source: owner

**Constraint.** GoRex runs on Linux, and only on Linux.
**Why.** The owner works on one Linux laptop, where Rex has no app, and
can test nothing else.
**Fails if.** A part of GoRex depends on a system the owner cannot test,
or a change is checked only on another system.

### B-8 Observe it without exposing the user

status: accepted · source: code, rex-docs

**Who.** The owner, and the agents that change GoRex.
**Needs.** To see what GoRex does in real use: to find faults and check
features.
**Why.** Agents build GoRex, and faults that tests miss show only in
real use.
**Fails if.** A fault in real use leaves no record to find it by, or the
record holds text the user typed or a program reported, or leaves the
machine.

### B-9 Built and kept by agents for one owner

status: rejected · source: code
rationale: project background, not a need; it is now the text at the top
of this file.
