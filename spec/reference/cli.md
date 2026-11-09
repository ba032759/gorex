# rex: the reference

Made from the code by `mise run spec-ref`; do not edit it. Every command
of `rex`, the flags it takes, and what its help leaves out. What the
commands are for is in `spec/functional/`; `spec check` reports a command
that no functional item names.

## `rex actions`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex block capture`

Also: `rex capture`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--format` | `--format` |
| `--trim` | `--trim` |
| `--unwrap` | `--unwrap` |

## `rex block close`

Also: `rex close`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |

## `rex block focus`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |

## `rex block inspect`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--json` | `--json` |

## `rex block list`

Also: `rex block ls`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--all` | `-A`, `--all` |
| `--json` | `--json` |
| `--quiet` | `-q`, `--quiet` |
| `--short` | `--short` |

## `rex block move`

Also: `rex move`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--ratio` | `--ratio` |

## `rex block new`

Flags: it reads its arguments itself.

## `rex block rename`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |

## `rex block send`

Also: `rex send`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |

## `rex block send-key`

Also: `rex send-key`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--action` | `--action` |

## `rex block swap`

Also: `rex swap`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |

## `rex block types`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex block zoom`

Also: `rex zoom`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--on` | `--on` |
| `--off` | `--off` |

## `rex client inspect`

Flags: it reads its arguments itself.

## `rex client list`

Also: `rex client ls`.

| Flag | Spellings |
|---|---|
| `--json` | `--json` |
| `--all` | `--all` |

## `rex completion`

Flags: none.

## `rex config check`

Flags: none.

## `rex config reload`

Flags: it reads its arguments itself.

## `rex do`

Help: `rex help do`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--block` | `-b`, `--block` |
| `--args` | `--args` |
| `--eval` | `-e`, `--eval` |
| `--action` | `-a`, `--action` |
| `--json` | `--json` |
| `--origin` | `--origin` |
| `--for` | `--for` |
| `--all` | `--all` |
| `--fail-fast` | `--fail-fast` |

Not in its help: `--session`, `--block`, `--eval`, `--action`, `--origin`, `--for`, `--all`, `--fail-fast`.

## `rex events`

Help: `rex help events`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--all` | `--all` |
| `--json` | `--json` |

## `rex focus`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--wrap` | `--wrap` |

## `rex keymap`

Help: `rex help keymap`.

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex kill`

Also: `rex session kill`.

Flags: none.

## `rex list`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex ls`

Also: `rex session list`, `rex session ls`.

| Flag | Spellings |
|---|---|
| `--json` | `--json` |
| `--quiet` | `-q`, `--quiet` |
| `--short` | `--short` |

## `rex new`

Also: `rex session new`.

| Flag | Spellings |
|---|---|
| `--dir` | `-c`, `--cwd`, `-d`, `--dir` |
| `--keep-open` | `--keep-open` |
| `--shell` | `--shell` |
| `--interactive` | `--interactive` |
| `--window` | `--window` |
| `--json` | `--json` |

## `rex new-tab`

Also: `rex window new`.

| Flag | Spellings |
|---|---|
| `--dir` | `-c`, `--cwd`, `-d`, `--dir` |
| `--keep-open` | `--keep-open` |
| `--shell` | `--shell` |
| `--interactive` | `--interactive` |
| `--session` | `-s`, `--session` |
| `--focus` | `--focus` |
| `--json` | `--json` |

## `rex resize`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--amount` | `--amount` |
| `--direction` | `--direction` |

## `rex run`

| Flag | Spellings |
|---|---|
| `--dir` | `-c`, `--cwd`, `-d`, `--dir` |
| `--keep-open` | `--keep-open` |
| `--shell` | `--shell` |
| `--interactive` | `--interactive` |
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--split` | `--split` |
| `--ratio` | `--ratio` |
| `--label` | `--label` |
| `--focus` | `--focus` |
| `--json` | `--json` |
| `--wait` | `--wait` |
| `--target` | `-t`, `--target` |
| `--below` | `-v`, `--vertical`, `--down`, `--below` |
| `--beside` | `--horizontal`, `--right`, `--beside` |

## `rex send-keys`

Flags: it reads its arguments itself.

## `rex server logs`

Flags: none.

## `rex server run`

Flags: none.

## `rex server start`

Flags: none.

## `rex server status`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex server stop`

| Flag | Spellings |
|---|---|
| `--forget` | `--forget` |

## `rex session inspect`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex session move`

| Flag | Spellings |
|---|---|
| `--before` | `--before` |
| `--after` | `--after` |
| `--index` | `--index` |

## `rex session rename`

Flags: none.

## `rex skill hook`

Help: `rex help skill`.

Flags: none.

## `rex skill remove`

Help: `rex help skill`.

| Flag | Spellings |
|---|---|
| `--project` | `--project` |
| `--dry-run` | `--dry-run` |
| `--force` | `--force` |
| `--session-naming` | `--session-naming` |
| `--json` | `--json` |

## `rex skill setup`

Help: `rex help skill`.

| Flag | Spellings |
|---|---|
| `--project` | `--project` |
| `--dry-run` | `--dry-run` |
| `--force` | `--force` |
| `--session-naming` | `--session-naming` |
| `--json` | `--json` |

## `rex skill show`

Help: `rex help skill`.

Flags: none.

## `rex skill status`

Help: `rex help skill`.

| Flag | Spellings |
|---|---|
| `--project` | `--project` |
| `--dry-run` | `--dry-run` |
| `--force` | `--force` |
| `--session-naming` | `--session-naming` |
| `--json` | `--json` |

## `rex split`

Help: `rex help split`.

| Flag | Spellings |
|---|---|
| `--dir` | `-c`, `--cwd`, `-d`, `--dir` |
| `--keep-open` | `--keep-open` |
| `--shell` | `--shell` |
| `--interactive` | `--interactive` |
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--split` | `--split` |
| `--ratio` | `--ratio` |
| `--label` | `--label` |
| `--focus` | `--focus` |
| `--json` | `--json` |
| `--wait` | `--wait` |
| `--target` | `-t`, `--target` |
| `--below` | `-v`, `--vertical`, `--down`, `--below` |
| `--beside` | `--horizontal`, `--right`, `--beside` |

Not in its help: `--target`, `--below`, `--beside`.

## `rex status`

Help: `rex help status`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--block` | `-b`, `--block` |
| `--app` | `--app` |
| `--msg` | `-m`, `--msg` |
| `--kind` | `--kind` |
| `--progress` | `--progress` |
| `--title` | `--title` |
| `--id` | `--id` |
| `--json` | `--json` |

Not in its help: `--session`, `--window`.

## `rex version`

| Flag | Spellings |
|---|---|
| `--json` | `--json` |

## `rex wait`

Help: `rex help wait`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--window` | `-w`, `--window` |
| `--for` | `--for` |
| `--json` | `--json` |

## `rex window close`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |

## `rex window focus`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--wrap` | `--wrap` |

## `rex window inspect`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--json` | `--json` |

## `rex window list`

Also: `rex window ls`.

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--all` | `-A`, `--all` |
| `--json` | `--json` |
| `--quiet` | `-q`, `--quiet` |
| `--short` | `--short` |

## `rex window move`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
| `--before` | `--before` |
| `--after` | `--after` |
| `--index` | `--index` |

## `rex window rename`

| Flag | Spellings |
|---|---|
| `--session` | `-s`, `--session` |
