# rex: the app's actions, the keymap, the server and agents

What these commands print, and where `rex skill setup` writes, are in
`technical/cli-app.md` (T-150, T-151). The data directory's files are in
`technical/files.md`. The flags each command takes are in
`reference/cli.md`.

## Actions of the app

### F-174 Perform an action of the app
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

`rex do ACTION [KEY=VALUE...]` SHALL ask the app of F-175 to perform the
action, as its key binding does, and wait for its answer.
The app SHALL perform the action where its own focus is, whatever `-s`
and `-b` name (Finding 36).
A value SHALL go to the app as a number when it is one, as a boolean when
it is `true` or `false`, and else as a string.
WHERE `--args` is given, the action SHALL also take the arguments of its
JSON object, and a `KEY=VALUE` argument SHALL take precedence over them.
WHEN `--json` is given, `rex do` SHALL print the app's answer.
IF no action is given or an argument is not `KEY=VALUE` or `--args` is
not a JSON object, THEN `rex` SHALL exit with 2.
IF the app does not have the action or fails to perform it, THEN `rex`
SHALL print the app's error and exit with 1.

### F-175 Find the app that performs an action
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex do` (F-174), `rex config reload` (F-180) and `rex client inspect`
(F-193) act through one app, which the server finds by these rules.

WHEN `-C` or `REX_CLIENT` names a client, the server SHALL send the
action to the app that it names, matched as a target (F-113).
WHEN no client is named and one app is connected, the server SHALL send
the action to that app.
IF no app is connected, THEN `rex` SHALL say that no app is connected
and exit with 1.
IF several apps are connected and none is named, THEN `rex` SHALL list
them and exit with 3.
IF the app does not answer in 30 seconds or goes away, THEN `rex` SHALL
exit with 1.

### F-176 Run a Rex script
status: proposed · traces: B-5 · source: code, rex-docs

GoRex runs no Lua (D-4). `rex do` takes as a script a first argument
that ends in `.lua`, holds `/` or `\`, or is `-`.

IF `rex do` gets `-e` or a script, THEN `rex` SHALL say that GoRex runs
no Lua and exit with 2.
`rex do` SHALL take Rex's `--origin`, `--for`, `--all`, `--fail-fast`
and `-a`, and ignore them (Finding 17).

### F-177 List the actions
status: proposed · traces: B-3 · source: code, rex-docs

`rex actions` SHALL list the actions of the app, sorted by name, each
with its title and the arguments it takes.
`rex actions` SHALL take the list from `rex` itself, without asking the
server or an app (Finding 19).

## Keys and configuration

### F-178 Show the keymap
status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex keymap` SHALL list the key bindings in effect, sorted by action,
each with its keys, its arguments and where it comes from.
The bindings SHALL be the preset's, then those of `keymap.json` in the
data directory.
A binding of the file SHALL replace the preset's binding of the same
keys.
The preset SHALL be the one that `keymap.json` names, else the default
of T-122.
`rex keymap` SHALL print the problems of `keymap.json` on standard
error.
IF a problem is an error, THEN `rex keymap` SHALL still exit with 0
(Finding 37).

### F-179 Check the keymap
status: proposed · traces: B-3 · source: code, rex-docs, tests · conflicts: F-178

`rex config check [PATH]` SHALL check `keymap.json` in the data
directory, or the file at `PATH`, without asking the app.
`rex config check` SHALL tell the file's path, whether it exists, its
preset, the number of bindings in effect and of those from the file, and
each problem.
A problem SHALL be a warning when a binding names an action that the app
does not have, and else an error.
IF a binding is wrong, THEN the keymap SHALL leave it out and keep the
other bindings.
IF a problem is an error, THEN `rex config check` SHALL exit with 1.

### F-180 Reload the keymap
status: proposed · traces: B-3 · source: code, rex-docs, tests, owner

`rex config reload` SHALL ask the app of F-175 to read its keymap again,
and then tell what `rex config check` tells (F-179).
IF no app is connected, THEN `rex config reload` SHALL exit with 1, as
`rex do` does (Finding 5).
IF a problem of the keymap is an error, THEN `rex config reload` SHALL
exit with 1.

## The server

### F-181 Tell whether the server runs
status: proposed · traces: B-2 · source: code, rex-docs

`rex server status` SHALL tell whether a server runs for the data
directory, and SHALL NOT start one.
WHEN a server runs, `rex server status` SHALL tell its process ID, how
long it has run, its socket, its data directory, its number of sessions
and its executable.
IF no server runs, THEN `rex server status` SHALL print `not running`
and exit with 4.

### F-182 Stop the server
status: proposed · traces: B-2 · source: code, rex-docs, tests, owner

`rex server stop` SHALL end the server and every program in its blocks,
after the server saves the model for the next server to restore (D-9).
WHERE `--discard` is given, `rex` SHALL delete the saved model once the
server has stopped, so that the next server starts with no session.
WHERE `--if-idle` is given, the server SHALL stop only when it has no
session and no client but this `rex`, and SHALL refuse a client that
connects as it stops.
IF `--if-idle` is given and the server is in use, THEN `rex` SHALL say
what uses it, leave the server running, and exit with 1.
IF `--if-idle` is given and the server is too old for it, THEN `rex`
SHALL leave the server running and exit with 4.
IF no server runs and `--discard` is not given, THEN `rex server stop`
SHALL exit with 4.
IF no server runs and `--discard` is given, THEN `rex` SHALL delete the
saved model and exit with 0.

### F-183 Start the server
status: proposed · traces: B-2 · source: code, rex-docs, tests, owner

`rex server start` SHALL start a server in the background when none
runs for the data directory, as T-123 says.
WHEN a server runs already, `rex server start` SHALL change nothing and
exit with 0.
IF the server does not answer within the timeout, THEN `rex server
start` SHALL exit with 4.
`rex server run` SHALL run a server in the foreground, until the server
stops.
IF another server runs for the data directory, THEN `rex server run`
SHALL exit with 1.

### F-430 Read the server's log
status: proposed · traces: B-2 · source: code, rex-docs, tests

`rex server logs` SHALL print the server's log, the whole of it, or with
`-n N` its newest `N` lines (Finding 38).
WHEN `--follow` is given, `rex` SHALL go on printing what the server
adds to its log, until the server stops.
WHILE `rex` follows the log, WHEN the log is replaced, `rex` SHALL print
the new log from its start.
IF the log does not exist, THEN `rex server logs` SHALL exit with 1.

### F-184 A server with nothing to do
status: proposed · traces: B-2 · source: code · kind: operational · verify: manual

WHEN a server has had no block and no client for 10 minutes, the server
SHALL exit.

The owner checks it by hand: start a server with no session, close every
client, wait 10 minutes; then `rex --autostart=false server status`
exits with 4 (Finding 21).

## Agents

### F-185 Set up agents
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill setup` SHALL install the skill for each agent named with
`--agent`, else for each agent found, where that agent reads its skills
(T-151).
WHEN no agent is named and none is found, `rex skill setup` SHALL
install the skill in the place that Codex, OpenCode and Pi share.
WHERE the agent is Claude Code, `rex skill setup` SHALL also add the
hooks of F-189 to its settings.
`rex skill setup` SHALL leave the rest of the settings as they were,
their keys in their order.
WHERE `--project` is given, `rex skill setup` SHALL install for every
agent in the repository the user is in, not in the user's home
directory.
IF a skill that `rex` did not install is in the place, THEN `rex skill
setup` SHALL leave it and report it as `foreign`, unless `--force` is
given.
WHERE `--dry-run` is given, `rex skill setup` SHALL change nothing and
report each change as one to make.
WHERE `--session-naming` is given, the hooks SHALL tell the agent to
name its session after its task, and a later setup SHALL keep this
choice.
`rex skill setup` SHALL report what it did to each skill and hook, and
a hook of Codex, OpenCode or Pi as `unsupported` (question 25).

### F-186 The hooks of Claude Code
status: superseded · superseded-by: F-185 · traces: B-4 · source: code, rex-docs, tests

`rex skill setup` SHALL add a hook to Claude Code's `settings.json` for
seven events: `SessionStart`, `UserPromptSubmit`, `Notification`,
`PermissionRequest`, `PostToolUse`, `Stop` and `SessionEnd`.
Each hook SHALL run `"$REX_BIN_DIR/rex" skill hook claude EVENT`, only
when `REX_BIN_DIR` is set.
`rex` SHALL leave the rest of the file as it was, its keys in their
order.
For Codex, OpenCode and Pi, the hook SHALL be `unsupported`.

### F-187 Safe setup
status: superseded · superseded-by: F-185 · traces: B-4 · source: code, rex-docs, tests

WHEN a `rex` skill that `rex` did not install is in the place THEN
`rex skill setup` SHALL leave it, and report `foreign`.
`--force` SHALL replace it.
`--dry-run` SHALL change nothing, and report `would be` and the state.
A second setup SHALL report `unchanged` for what is current.
`--session-naming` SHALL tell the agent, at its start, to name the
session after its task; later setups SHALL keep this choice.

### F-188 Tell what is installed
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill status` SHALL report, for every agent or each one named with
`--agent`, whether it is found and the state of its skill and its hook.
`rex skill status` SHALL change nothing.
The state SHALL be `current`, `outdated`, `missing`, `foreign`,
`unreadable` or `unsupported`.
WHERE `--project` is given, `rex skill status` SHALL report on the
repository the user is in.

### F-431 Remove the skill
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill remove` SHALL remove the skill files and the hooks that `rex`
installed, for every agent or each one named with `--agent`.
`rex skill remove` SHALL leave a skill that `rex` did not install, and
every other hook and setting.
WHERE `--dry-run` is given, `rex skill remove` SHALL change nothing and
report each removal as one to make.
WHERE `--project` is given, `rex skill remove` SHALL remove them from
the repository the user is in.

### F-432 Print the skill's guide
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill show` SHALL print the guide that the skill holds.
The guide SHALL say session, window and block.
The guide SHALL use the app's own words for a window and a block only to
say how the app shows them (Finding 13).

### F-189 Report an agent's status from its hooks
status: proposed · traces: B-4 · source: code, tests

Claude Code runs `rex skill hook claude EVENT` at its events, as F-185
installs it.

WHEN `rex skill hook` runs outside a GoRex block, `rex` SHALL do nothing
and exit with 0.
WHEN the event is `SessionStart`, `rex` SHALL tell the agent its
session's and its block's IDs, and with `--session-naming`, to name the
session after its task.
`rex` SHALL report the agent's program status to its block, as this
table holds:

| Event | Status |
|---|---|
| `SessionStart` | `idle` |
| `UserPromptSubmit` | `working`, with the prompt |
| `Notification` of a permission, `PermissionRequest` | `blocked`, kind `permission` |
| `Notification` of a question | `blocked`, kind `question` |
| another `Notification` | none |
| `PostToolUse` | `working` |
| `Stop` | `done`, with the agent's last text |
| `SessionEnd` | `clear`, unless the status is `done` or `error` |

The hook SHALL always exit with 0.
