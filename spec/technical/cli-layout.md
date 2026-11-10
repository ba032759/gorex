# Technical: what the layout commands print, and how a block's program starts and ends

### T-124 The output of the layout commands

status: accepted · traces: F-130, F-131, F-134, F-135, F-150, F-156, F-190, F-193 · source: code, rex-docs, tests

Tables of sessions, windows and blocks have a first column for the marks
of F-117. The columns are:

| Command         | Columns                                                                                                                           |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `rex ls`        | `ID`, `LABEL`, `WINDOWS`, `BLOCKS`, `CREATED` (`N ago`)                                                                           |
| `rex window ls` | `#` (position from 1), `ID`, `LABEL`, `SESSION`, `BLOCKS`, `FOCUSED`                                                              |
| `rex block ls`  | `ID`, `LABEL`, `WINDOW` (position), `PROGRAM`, `STATUS` (`state(kind)` or `-`), `PID`, `SIZE` (`COLSxROWS`), `DIR` (`~` for home) |
| `rex client ls` | `ID`, `KIND`, `NAME`, `AGE`, `VIEWING`                                                                                            |

With `--json`:

| Command                        | Prints                                                                                         |
| ------------------------------ | ---------------------------------------------------------------------------------------------- |
| `rex ls`, `rex session ls`     | `{"sessions": [SESSION...]}`                                                                   |
| `rex session inspect`          | `SESSION`                                                                                      |
| `rex window ls`                | `{"windows": [WINDOW...]}`, each with `session_id`                                             |
| `rex window inspect`           | `WINDOW`, with `session_id`                                                                    |
| `rex block ls`                 | `{"blocks": [BLOCK...]}`                                                                       |
| `rex block inspect`            | `BLOCK`                                                                                        |
| `rex list`                     | `{"model": MODEL, "blocks": [BLOCK...]}`                                                       |
| `rex new`                      | `session_id`, `window_id`, `block_id`, `block_ids`, `label`, `revision`                        |
| `rex window new`, `rex run`    | `session_id`, `window_id`, `block_id`, `block_ids`, `revision`                                 |
| `rex split`, `rex run --split` | `block_id`, `block_ids`, `split_ids`, `anchor_block_id`, `window_id`, `session_id`, `revision` |
| `rex block types`              | `{"types": [{"flavor", "title"}]}`                                                             |
| `rex client ls`                | `{"clients": [CLIENT...]}`, every kind                                                         |
| `rex client inspect`           | `CLIENT`                                                                                       |

- `SESSION` is `session_id`, `label`, `created`, `active_window_id` and
  `windows`.
- `WINDOW` is `window_id`, `label`, `index` from 1, `block_ids`,
  `focused_block_id`, `zoomed_block_id` and `layout` (T-107).
- `BLOCK` is a block's state (T-108), with `session_label`,
  `window_index` and `focused`.
- `MODEL` is the model (T-107).
- `CLIENT` is `client_id`, `kind`, `name`, `pid`, `connected_at` and
  `viewing_session_id`.

### T-125 How a block's program starts and ends

status: accepted · traces: F-133, F-140 · source: code

The shell is `--shell`, else `SHELL`, else the system's default, found on
`PATH` when it is not absolute.

- No command: the shell, as a login shell (`argv[0]` is `-` and its
  name, as `-zsh`).
- A command, and the shell is `sh`, `bash`, `zsh`, `dash`, `ksh`, `mksh`,
  `ash`, `yash` or `fish`: `SHELL -l [-i] -c 'exec "$@"' NAME ARGS...`;
  fish takes `exec $argv` and no `NAME`. `-i` is `--interactive`.
- A command, and another shell or `--shell none`: the command, found on
  `PATH`, started directly.

The program runs in a session of its own, with the block's terminal as
its controlling terminal. To close a block, the server sends `SIGHUP` to
that session's process group and to the terminal's foreground group,
then `SIGKILL` to the process group after 1 second. A program a signal
ended has the exit status 128 plus the signal's number.

### T-126 What inspect prints

status: accepted · traces: F-190 · source: code

Without `--json`, the inspect commands print one field a line:

| Command               | Fields                                                                                                                                                                                                                                                                                                                         |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `rex session inspect` | `session` (ID), `label`, `created` (RFC 3339), `windows` (their number), then a line for each window: the mark of F-117, position, short ID, label, number of blocks                                                                                                                                                           |
| `rex window inspect`  | `window` (ID), `label`, `session` (ID and name), `index` (from 1), `focused`, `zoomed` when a block is, then `blocks`: each block's ID and label                                                                                                                                                                               |
| `rex block inspect`   | `block`, `label`, `session`, `window` (ID and position), `focused`, `pid`, `shell`, `program` (with its arguments), `idle`, `dir`, `size`, `title` when set, `output` (bytes, and how long ago the last), `attached` (clients), `exited` (the exit status) when the program exited, and a `status` line for each status record |

With `--json` they print what T-124 says.
