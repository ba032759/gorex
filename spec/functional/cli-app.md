# rex: the app's actions, the keymap, the server and agents

## Actions of the app

### F-174 Perform an action of the app
status: proposed · traces: B-3, B-5 · source: code, rex-docs, tests

`rex do ACTION [KEY=VALUE...]` SHALL ask an app to perform the action,
as its key binding does.
A value that is a number SHALL go as a number; `true` and `false` as
booleans; anything else as a string.
`--args JSON` SHALL give more arguments: inline, `@FILE`, or `-` for
standard input; `KEY=VALUE` arguments SHALL take precedence.
`--json` SHALL print the app's answer.
WHEN an argument is not `KEY=VALUE`, or `--args` is not a JSON object,
THEN `rex` SHALL exit with 2.

### F-175 The app that performs it
status: proposed · traces: B-3 · source: code, rex-docs, tests

WHEN `-C` or `REX_CLIENT` names a client THEN the server SHALL send the
action to the app it matches, by ID or name.
WHEN one app is connected THEN the server SHALL send it there.
WHEN no app is connected THEN `rex` SHALL exit with 1: `no app is
connected: open GoRex`.
WHEN several apps are connected and none is named THEN `rex` SHALL exit
with 3 and list them.
WHEN the app does not answer in 30 seconds THEN `rex` SHALL exit with 1.

### F-176 No scripts
status: proposed · traces: B-5 · source: code, rex-docs

WHEN `rex do` gets `-e`, or a first argument that ends in `.lua`, holds
`/` or `\`, or is `-`, THEN `rex` SHALL exit with 2.
Its message SHALL say that GoRex runs no Lua.
`--origin`, `--for`, `--all`, `--fail-fast` and `-a` SHALL be taken and
change nothing.

### F-177 List the actions
status: proposed · traces: B-3 · source: code, rex-docs

`rex actions` SHALL print the app's actions, sorted by name, with the
columns `NAME`, `TITLE` and `ARGS`.
`--json` SHALL print `{"actions": [...]}`.
`rex` SHALL take the list from itself, without asking the server or an
app.

## Keys and configuration

### F-178 Show the keymap
status: proposed · traces: B-3 · source: code, rex-docs

`rex keymap` SHALL print the key bindings in effect, sorted by action,
with the columns `KEY`, `ACTION`, `ARGS` and `SOURCE`.
The bindings SHALL be the preset's, then those of `keymap.json` in the
data directory.
The preset SHALL follow `settings.json` in the data directory.
`rex keymap` SHALL print the problems of the file on standard error.
`--json` SHALL print `path`, `bindings` and `problems`.

### F-179 Check the keymap
status: proposed · traces: B-3 · source: code, rex-docs

`rex config check [PATH]` SHALL read `keymap.json`, or the file at
`PATH`.
It SHALL print the path, whether the file exists, the preset, the number
of bindings, and each problem.
WHEN a problem is an error THEN `rex` SHALL exit with 1.

### F-180 Reload the keymap
status: proposed · traces: B-3 · source: code, rex-docs

`rex config reload` SHALL ask the app to read its keymap again, then
print what `rex config check` prints.
WHEN no app is connected THEN `rex` SHALL print that the app reads the
keymap as it starts, and exit with 0.

## The server

### F-181 Server status
status: proposed · traces: B-2 · source: code, rex-docs

`rex server status` SHALL print whether a server runs, with its process
ID, uptime, socket, data directory, number of sessions and executable.
`--json` SHALL print `running`, `pid`, `started`, `protocol`, `socket`,
`dir`, `exe` and `sessions`.
WHEN no server runs THEN `rex` SHALL print `not running` and exit with 4.

### F-182 Stop the server
status: proposed · traces: B-2 · source: code, rex-docs, tests

`rex server stop` SHALL end the server and every program in its blocks.
The server SHALL save the model first: the next server restores it
(D-9).
`--forget` SHALL delete the saved state, so that the next server starts
with no session.
WHEN no server runs THEN `rex` SHALL exit with 4.

### F-183 Start the server, read its log
status: proposed · traces: B-2 · source: code, rex-docs

`rex server start` SHALL start a server when none runs, and exit with 0
when one runs.
`rex server run` SHALL run a server in the foreground.
WHEN another server runs for the data directory THEN `rex server run`
SHALL exit with 1.
`rex server logs` SHALL print the server's log, `server.log` in the data
directory.

### F-184 A server with nothing to do
status: proposed · traces: B-2 · source: code

WHEN a server has had no block and no client for 10 minutes THEN it
SHALL exit.

## Agents

### F-185 Set up agents
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill setup` SHALL install the skill for each agent named with
`--agent`, else for each agent found, else at `~/.agents/skills/rex`.
An agent SHALL count as found when its directory exists or its program
is on `PATH`.
Claude Code's skill SHALL go to `CLAUDE_CONFIG_DIR/skills/rex`, else
`~/.claude/skills/rex`.
The skill of Codex, OpenCode and Pi SHALL go to `~/.agents/skills/rex`.
`--project` SHALL put them in the repository the user is in.
For each item, `rex` SHALL print its agent, its path and its state:
`created`, `updated`, `unchanged`, `foreign` or `unreadable`.

### F-186 The hooks of Claude Code
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill setup` SHALL add a hook to Claude Code's `settings.json` for
seven events: `SessionStart`, `UserPromptSubmit`, `Notification`,
`PermissionRequest`, `PostToolUse`, `Stop` and `SessionEnd`.
Each hook SHALL run `"$REX_BIN_DIR/rex" skill hook claude EVENT`, only
when `REX_BIN_DIR` is set.
`rex` SHALL leave the rest of the file as it was, its keys in their
order.
For Codex, OpenCode and Pi, the hook SHALL be `unsupported`.

### F-187 Safe setup
status: proposed · traces: B-4 · source: code, rex-docs, tests

WHEN a `rex` skill that `rex` did not install is in the place THEN
`rex skill setup` SHALL leave it, and report `foreign`.
`--force` SHALL replace it.
`--dry-run` SHALL change nothing, and report `would be` and the state.
A second setup SHALL report `unchanged` for what is current.
`--session-naming` SHALL tell the agent, at its start, to name the
session after its task; later setups SHALL keep this choice.

### F-188 Skill status, removal and guide
status: proposed · traces: B-4 · source: code, rex-docs, tests

`rex skill status` SHALL report the skill and hook of every agent as
`current`, `outdated`, `missing`, `foreign`, `unreadable` or
`unsupported`.
`rex skill remove` SHALL remove only the skill files and hooks that
`rex` installed, for every agent; a foreign skill SHALL stay.
`rex skill show` SHALL print the guide that the skill holds.

### F-189 The hook of an agent
status: proposed · traces: B-4 · source: code, tests

`rex skill hook claude EVENT` SHALL do nothing and exit with 0 outside a
GoRex block.
At `SessionStart` it SHALL print the session's and block's IDs for the
agent.
It SHALL report the agent's program status to its block:

| Event | Status |
|---|---|
| `SessionStart` | `idle` |
| `UserPromptSubmit` | `working`, with the prompt |
| `Notification` of a permission, `PermissionRequest` | `blocked`, kind `permission` |
| `Notification` of a question | `blocked`, kind `question` |
| `PostToolUse` | `working` |
| `Stop` | `done`, with the agent's last text |
| `SessionEnd` | `clear`, unless the status is `done` or `error` |

The hook SHALL always exit with 0.
