# rex: the reference

Made from the code by `mise run spec-ref`; do not edit it. Every command
of `rex`, the flags it takes, the values they take and their defaults,
as the commands declare them; `rex help COMMAND` prints the same. What the
commands are for is in `spec/functional/`; `spec check` reports a command
that no functional item names.

## Options before the command

| Flag | Value | Default | Help |
|---|---|---|---|
| `-S`, `--server` |  | none | Rex's remote server: refused, GoRex's server runs on this machine |
| `-C`, `--client` | `CLIENT` | REX_CLIENT | the app, CLIENT, that rex do asks |
| `--timeout` | `DURATION`, a duration, as 300ms, 10s, 2h45m | `10s` | wait up to DURATION for an answer |
| `--autostart` |  | `true` | start a server when none runs; --autostart=false does not |
| `--color` | `WHEN` | none | Rex's WHEN; taken, and ignored |
| `-v`, `--verbose`, `--vv` |  | none | Rex's; taken, and ignored |

## `rex actions`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex block capture`

Also: `rex capture`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--format` | `FORMAT`, text or vt | `text` | the FORMAT: text, or vt to keep the escape sequences |
| `--trim` |  | `false` | drop trailing blank lines |
| `--unwrap` |  | `false` | join the rows of a line the terminal wrapped |

## `rex block close`

Also: `rex close`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |

## `rex block focus`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |

## `rex block inspect`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--json` |  | `false` | print JSON |

## `rex block ls`

Also: `rex block list`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-A`, `--all` |  | `false` | every session's |
| `--json` |  | `false` | print JSON |
| `-q`, `--quiet` |  | `false` | print the IDs alone, one a line |
| `--short` |  | `true` | short IDs; --short=false prints full ones |

## `rex block move`

Also: `rex move`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `--ratio` | `PCT`, from 1 to 99 | `50` | the block's share of its new split, in percent: PCT |

## `rex block new`

Flags: none; it reads its arguments itself.

## `rex block rename`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |

## `rex block send`

Also: `rex send`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |

## `rex block send-key`

Also: `rex send-key`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--action` | `ACTION` | none | press, or release, which sends nothing: Rex's key ACTION |

## `rex block swap`

Also: `rex swap`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |

## `rex block types`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex block zoom`

Also: `rex zoom`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--on` |  | `false` | zoom it |
| `--off` |  | `false` | end its zoom |

## `rex client inspect`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex client ls`

Also: `rex client list`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |
| `--all` |  | `false` | rex's commands too |

## `rex completion`

Flags: none; it reads its arguments itself.

## `rex config check`

Flags: none.

## `rex config reload`

Flags: none; it reads its arguments itself.

## `rex do`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-b`, `--block` | `BLOCK` | none | the BLOCK to act on |
| `--args` | `JSON` | none | the action's arguments as a JSON object; - reads stdin, @FILE a file |
| `-e`, `--eval` | `CODE` | none | Rex's Lua CODE: refused |
| `-a`, `--action` | `ACTION` | none | Rex's ACTION; taken, and ignored |
| `--json` |  | `false` | print JSON |
| `--origin` | `ORIGIN` | none | Rex's ORIGIN; taken, and ignored |
| `--for` | `DURATION` | none | Rex's DURATION; taken, and ignored |
| `--all` |  | `false` | Rex's; taken, and ignored |
| `--fail-fast` |  | `false` | Rex's; taken, and ignored |

## `rex events`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `--all` |  | `false` | every event |
| `--json` |  | `false` | print JSON |

## `rex focus`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `--wrap` |  | `false` | from the last, go on to the first |

## `rex keymap`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex list`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex resize`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--amount` | `PCT`, above 0 | `5` | how far to move it, PCT percent of the split |
| `--direction` | `DIRECTION` | none | the DIRECTION, when no argument gives it |

## `rex run`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-c`, `--cwd`, `-d`, `--dir` | `DIR` | the directory rex runs in | start in DIR; ~ is your home |
| `--keep-open` |  | `false` | keep the terminal after its command exits |
| `--shell` | `SHELL` | your login shell | run the command in SHELL's login environment; none: directly |
| `--interactive` |  | `false` | also read the shell's interactive startup files |
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--split` | `DIRECTION` | a new tab | put the block beside the anchor, toward DIRECTION: right, left, above or below |
| `--ratio` | `PCT`, from 1 to 99 | `50` | the new block's share of the split, in percent: PCT |
| `--label` | `LABEL` | none | a LABEL to target it by |
| `--focus` |  | `true` | focus it; --focus=false leaves the focus where it is |
| `--json` |  | `false` | print JSON |
| `--wait` |  | `false` | wait for the command, and exit with its status |
| `-t`, `--target` | `BLOCK` | none | GoRex's first --block BLOCK |
| `-v`, `--vertical`, `--down`, `--below` |  | `false` | GoRex's first --split=below |
| `--horizontal`, `--right`, `--beside` |  | `false` | GoRex's first --split=right |

## `rex send-keys`

Flags: none; it reads its arguments itself.

## `rex server logs`

Flags: none; it reads its arguments itself.

## `rex server run`

Flags: none; it reads its arguments itself.

## `rex server start`

Flags: none; it reads its arguments itself.

## `rex server status`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex server stop`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--forget` |  | `false` | also forget the sessions |

## `rex session inspect`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex session kill`

Also: `rex kill`.

Flags: none.

## `rex session ls`

Also: `rex session list`, `rex ls`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |
| `-q`, `--quiet` |  | `false` | print the IDs alone, one a line |
| `--short` |  | `true` | short IDs; --short=false prints full ones |

## `rex session move`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--before` | `TARGET` | none | put it before TARGET |
| `--after` | `TARGET` | none | put it after TARGET |
| `--index` | `N`, from 1 | none | put it at position N, from 1 |

## `rex session new`

Also: `rex new`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-c`, `--cwd`, `-d`, `--dir` | `DIR` | the directory rex runs in | start in DIR; ~ is your home |
| `--keep-open` |  | `false` | keep the terminal after its command exits |
| `--shell` | `SHELL` | your login shell | run the command in SHELL's login environment; none: directly |
| `--interactive` |  | `false` | also read the shell's interactive startup files |
| `--window` | `LABEL` | none | a LABEL for its window |
| `--json` |  | `false` | print JSON |

## `rex session rename`

Flags: none.

## `rex skill hook`

Flags: none; it reads its arguments itself.

## `rex skill remove`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--agent` | `AGENT`, again for more | every agent found | only this AGENT |
| `--project` |  | `false` | in this project, not in your home |
| `--dry-run` |  | `false` | tell what it would do, and do nothing |
| `--force` |  | `false` | replace a skill of the same name that rex did not install |
| `--session-naming` |  | `false` | also have the agent name its session after its task |
| `--json` |  | `false` | print JSON |

## `rex skill setup`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--agent` | `AGENT`, again for more | every agent found | only this AGENT |
| `--project` |  | `false` | in this project, not in your home |
| `--dry-run` |  | `false` | tell what it would do, and do nothing |
| `--force` |  | `false` | replace a skill of the same name that rex did not install |
| `--session-naming` |  | `false` | also have the agent name its session after its task |
| `--json` |  | `false` | print JSON |

## `rex skill show`

Flags: none; it reads its arguments itself.

## `rex skill status`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--agent` | `AGENT`, again for more | every agent found | only this AGENT |
| `--project` |  | `false` | in this project, not in your home |
| `--dry-run` |  | `false` | tell what it would do, and do nothing |
| `--force` |  | `false` | replace a skill of the same name that rex did not install |
| `--session-naming` |  | `false` | also have the agent name its session after its task |
| `--json` |  | `false` | print JSON |

## `rex split`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-c`, `--cwd`, `-d`, `--dir` | `DIR` | the anchor's | start in DIR; ~ is your home |
| `--keep-open` |  | `false` | keep the terminal after its command exits |
| `--shell` | `SHELL` | your login shell | run the command in SHELL's login environment; none: directly |
| `--interactive` |  | `false` | also read the shell's interactive startup files |
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--split` | `DIRECTION` | `right` | the DIRECTION of the new block: right, left, above or below |
| `--ratio` | `PCT`, from 1 to 99 | `50` | the new block's share of the split, in percent: PCT |
| `--label` | `LABEL` | none | a LABEL to target it by |
| `--focus` |  | `true` | focus it; --focus=false leaves the focus where it is |
| `--json` |  | `false` | print JSON |
| `--wait` |  | `false` | wait for the command, and exit with its status |
| `-t`, `--target` | `BLOCK` | none | GoRex's first --block BLOCK |
| `-v`, `--vertical`, `--down`, `--below` |  | `false` | GoRex's first --split=below |
| `--horizontal`, `--right`, `--beside` |  | `false` | GoRex's first --split=right |

## `rex status`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `-b`, `--block` | `BLOCK` | yours, else the window's focused block | the BLOCK to act on |
| `--app` | `APP` | none | the APP that reports |
| `-m`, `--msg` | `TEXT` | none | a message: TEXT |
| `--kind` | `KIND` | none | what it waits for, the KIND: permission, question or auth |
| `--progress` | `PCT` | none | how far it is, PCT percent |
| `--title` | `TEXT` | none | a title: TEXT |
| `--id` | `ID` | none | the ID of the record, for a program that reports several |
| `--json` |  | `false` | print JSON |

## `rex version`

| Flag | Value | Default | Help |
|---|---|---|---|
| `--json` |  | `false` | print JSON |

## `rex wait`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-w`, `--window` | `WINDOW` | none | the WINDOW to act in |
| `--for` | `DURATION`, a duration, as 300ms, 10s, 2h45m | no limit | give up after DURATION, with status 1 |
| `--json` |  | `false` | print JSON |

## `rex window close`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |

## `rex window focus`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `--wrap` |  | `false` | from the last, go on to the first |

## `rex window inspect`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `--json` |  | `false` | print JSON |

## `rex window ls`

Also: `rex window list`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `-A`, `--all` |  | `false` | every session's |
| `--json` |  | `false` | print JSON |
| `-q`, `--quiet` |  | `false` | print the IDs alone, one a line |
| `--short` |  | `true` | short IDs; --short=false prints full ones |

## `rex window move`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
| `--before` | `TARGET` | none | put it before TARGET |
| `--after` | `TARGET` | none | put it after TARGET |
| `--index` | `N`, from 1 | none | put it at position N, from 1 |

## `rex window new`

Also: `rex new-tab`.

| Flag | Value | Default | Help |
|---|---|---|---|
| `-c`, `--cwd`, `-d`, `--dir` | `DIR` | with a command, the directory rex runs in; else the focused block's | start in DIR; ~ is your home |
| `--keep-open` |  | `false` | keep the terminal after its command exits |
| `--shell` | `SHELL` | your login shell | run the command in SHELL's login environment; none: directly |
| `--interactive` |  | `false` | also read the shell's interactive startup files |
| `-s`, `--session` | `SESSION` | none | the SESSION to open it in |
| `--focus` |  | `true` | make it the active window; --focus=false leaves the focus where it is |
| `--json` |  | `false` | print JSON |

## `rex window rename`

| Flag | Value | Default | Help |
|---|---|---|---|
| `-s`, `--session` | `SESSION` | none | the SESSION to act in |
