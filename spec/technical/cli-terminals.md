# Technical: the bytes of a key, and what the terminal commands print

### T-160 The bytes of a key

status: accepted · traces: F-161, F-162 · source: code, tests

The server sends xterm's bytes for each key of F-162:

| Key                                                             | Bytes                                                                                                          |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `enter`, `tab`, `escape`                                        | CR, HT, ESC                                                                                                    |
| `backspace`, `ctrl+backspace`                                   | DEL, BS                                                                                                        |
| `space`, `ctrl+space`                                           | SP, NUL                                                                                                        |
| `shift+tab`                                                     | `ESC [ Z`                                                                                                      |
| a letter with `ctrl`                                            | its control character, 1 to 26                                                                                 |
| `ctrl` with `@ 2`, `[ 3`, `\ 4`, `] 5`, `^ 6`, `_ - 7 /`, `? 8` | NUL, ESC, FS, GS, RS, US, DEL                                                                                  |
| a character with `shift`                                        | the character a US keyboard types: `A`, `!`, `{`                                                               |
| anything with `alt`                                             | ESC before the key's bytes                                                                                     |
| arrows, `home`, `end`                                           | `ESC [ A` to `D`, `H`, `F`; `ESC O` and the letter when the program asked for application cursor keys (DECCKM) |
| modified arrows, `home`, `end`, `f1`–`f4`                       | `ESC [ 1 ; M` and the letter, M being 1 plus shift 1, alt 2, ctrl 4                                            |
| `insert`, `delete`, `pageup`, `pagedown`                        | `ESC [ 2 ~`, `3`, `5`, `6`, with `; M` before `~` when modified                                                |
| `f1`–`f4`                                                       | `ESC O P` to `ESC O S`                                                                                         |
| `f5`–`f12`                                                      | `ESC [ 15 ~`, `17`, `18`, `19`, `20`, `21`, `23`, `24`, with `; M` when modified                               |

`super` adds nothing. The keys of one `rex send-key` go to the terminal
in one write.

### T-161 What the terminal commands print

status: accepted · traces: F-164, F-166, F-169, F-170, F-173 · source: code, tests

| Command             | Prints                                                                                                          |
| ------------------- | --------------------------------------------------------------------------------------------------------------- |
| `rex capture`       | the text, or the escape sequences; it has no `--json`                                                           |
| `rex wait --json`   | one line: `block_id`, `window_id` or `session_id`, and `exit_code`                                              |
| `rex events`        | a line for each event: the local time `HH:MM:SS`, the type, the short ID of the item, and the details           |
| `rex events --json` | each event of T-106, one line each                                                                              |
| `rex status`        | a line for each record: `ID  STATE (KIND) PCT%  APP  TITLE: MSG  AGE`, each part only when set                  |
| `rex status --json` | `block_id`, and `records`: `id`, `state`, `kind`, `progress` (`-1` when none), `app`, `title`, `msg`, `updated` |

In a line of `rex events`, the item is the first of `block_id`,
`window_id`, `session_id` and `client_id` that the event has. The
details are `label` and `revision` when the event has them, the
payload as JSON for a terminal event, and `session` and its short ID for
a session event.

An event of a session is `{"type": "session_event", "session_id",
"payload": {"session_id", "event": {"type", "payload"}}}`. A terminal
event is a session event of type `block_event`, whose payload is
`block_id`, `creator_name` (`com.superlogical.terminal`), `name` and
`payload`. The payloads:

| Event                                    | Payload                                                                     |
| ---------------------------------------- | --------------------------------------------------------------------------- |
| `session_created`                        | `session`: `session_id`, `label`                                            |
| `session_destroyed`                      | `session_id`                                                                |
| `client_joined`                          | `client`: `client_id`, `kind`, `name`, `connected_at`, `viewing_session_id` |
| `client_left`                            | `client_id`                                                                 |
| `window_created`, `window_label_changed` | `window_id`, `label`                                                        |
| `window_closed`, `active_window_changed` | `window_id`                                                                 |
| `block_created`, `block_closed`          | `block_id`                                                                  |
| `session_label_changed`                  | `label`                                                                     |
| `session_view_changed`                   | `revision`                                                                  |
| `child_exited`                           | `exit_code`, and `error` when it is not 0                                   |
| `bell`                                   | nothing                                                                     |
| `title_changed`                          | `title`                                                                     |
| `pwd_changed`                            | `pwd`                                                                       |
| `process_changed`                        | `foreground`, `args`                                                        |
| `desktop_notification`                   | `title`, `body`                                                             |
| `program_status_changed`                 | `id`, `record`, `reason`                                                    |
| `program_status_removed`                 | `ids`, `reason`                                                             |

`reason` is `report`, `prompt`, `command` or `reset` (F-460, F-461).
