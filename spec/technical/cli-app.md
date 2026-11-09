# Technical: what the app, keymap, server and skill commands print, and where the skill goes

### T-150 The output of the app, keymap and server commands
status: accepted · traces: F-177, F-178, F-179, F-180, F-181 · source: code, rex-docs, tests

A table has a header line and columns separated by two spaces or more.
`-` stands for an empty value.

| Command | Text | `--json` |
|---|---|---|
| `rex actions` | `NAME`, `TITLE`, `ARGS` | `{"actions": [{"name", "title", "args"}]}`; `args` only when the action takes some |
| `rex keymap` | `KEY`, `ACTION`, `ARGS` (JSON), `SOURCE`; the problems on standard error | `path`, `bindings` (`key`, `action`, `args`, `source`), `problems` |
| `rex config check`, `rex config reload` | lines `path:`, `exists: yes` or `no`, `preset:`, `bindings: N (M from the file)`, then one line for each problem | `path`, `exists`, `preset`, `bindings`, `from_file`, `problems` (a list, empty when none) |
| `rex server status` | `running, pid N, up DURATION`, then `socket`, `state` (the data directory), `sessions` and `exe` lines | `running`, `pid`, `started`, `protocol`, `socket`, `dir`, `exe`, `sessions` |

A binding's `SOURCE` is `default` for the preset's, else
`keymap.json:bindings[I]`. A problem is a line that starts with `error`
or `warning`, then the file or the binding, then what is wrong. The
preset of `rex config check` ends in ` (default)` when the file names
none.

With no server, `rex server status` prints `not running (socket PATH)`,
and with `--json`, `{"running": false, "socket": PATH}`.

### T-151 Where rex skill setup writes
status: accepted · traces: F-185, F-188, F-189, F-431, F-432 · source: code, rex-docs, tests

| Agent | Found by | Skill | Hooks |
|---|---|---|---|
| Claude Code | `CLAUDE_CONFIG_DIR`, else `~/.claude`; or `claude` on `PATH` | `skills/rex` in that directory | `settings.json` in that directory |
| Codex | `CODEX_HOME`, else `~/.codex`; or `codex` | `~/.agents/skills/rex` | none |
| OpenCode | `~/.config/opencode`; or `opencode` | `~/.agents/skills/rex` | none |
| Pi | `~/.pi`; or `pi` | `~/.agents/skills/rex` | none |

With `--project`, `~` and Claude Code's directory are the repository's
root (the nearest directory above the working directory with `.git`,
else the working directory): `.claude/skills/rex`,
`.claude/settings.json` and `.agents/skills/rex`. Agents that share a
directory share one skill.

The skill is one file, `SKILL.md`, with the marker `<!-- installed by
rex skill setup (GoRex) -->` after its front matter, then the guide that
`rex skill show` prints. A `SKILL.md` without the marker, or a non-empty
directory without `SKILL.md`, is foreign.

Each of the seven events `SessionStart`, `UserPromptSubmit`,
`Notification`, `PermissionRequest`, `PostToolUse`, `Stop` and
`SessionEnd` gets, in `hooks`, an entry whose one command is:

```sh
[ -n "$REX_BIN_DIR" ] && "$REX_BIN_DIR/rex" skill hook claude EVENT || true
```

`SessionStart`'s adds `--session-naming` when it is chosen. A hook is
`rex`'s when its command holds `rex` and ` skill hook `; setup replaces
it, status compares it, remove deletes it, and every other entry stays.
The hook reads Claude Code's JSON on standard input (`prompt`,
`message`, `notification_type`, `tool_name`, `last_assistant_message`,
`transcript_path`) and sends an OSC 7501 report (T-110) to the server,
waiting at most 2 seconds for each request. A `Notification` is a
question when its type is `elicitation_dialog`, and a permission when
its type is `permission_prompt` or its message names a permission.

A report of setup, status or remove lists, for each agent, its skill and
its hook: the state and the path; `--json` prints `{"items": [{"agent",
"item", "state", "path"}], "notes": [...]}`. The states are `created`,
`updated`, `unchanged`, `foreign`, `unreadable` and `unsupported` for
setup, with `would be ` before them under `--dry-run`; those of F-188
for status, where the agent ends in ` (found)` or ` (not found)`; and
`removed` (`would be removed`), `absent` and `foreign, kept` for
remove.
