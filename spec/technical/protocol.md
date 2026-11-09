# Technical: the protocol between rex and the server

What another implementation of `rex` keeps to talk to GoRex's server.
Protocol version 2.

### T-100 The socket
status: proposed · traces: F-106, F-107 · source: code

The server listens on a Unix socket (F-106), mode `0600`.
A connection's first line decides what it is.
A line `{"op":"attach","sid":BLOCK,"cols":N,"rows":N}` makes it a
terminal stream, which the app uses and `rex` does not.
Any other line makes it a control connection (T-101).

### T-101 Control connection
status: proposed · traces: F-104, F-110 · source: code, tests

A control connection carries one JSON object per line, both ways.
A request is `{"id": N, "op": OP, "args": {...}}`; `id` rises from 1 for
each request of a connection.
Older requests also use `sid`, `cols`, `rows` and `create` beside `args`.
A response is `{"id": N, "error": TEXT, "code": CODE, "data": ...}`, with
`error` and `code` only on failure.
The server answers `kill`, `wait` and `client.do` out of order; a client
matches answers by `id`.
An event is a line `{"id": 0, "event": {...}}` (T-106).

### T-102 Hello
status: proposed · traces: F-108, F-123, F-181 · source: code, tests

A client sends `hello` first.
Its `args` say who it is: `kind` (`app` or `cli`, the default) and
`name`.
The data are `version` (the protocol, 2), `pid`, `started`, `host`,
`exe`, `exeTime`, `clientID` and `bin`.
A client that gets another `version` stops (F-108).

### T-103 Error codes
status: proposed · traces: F-104, F-113 · source: code, tests

| `code` | Meaning | `rex` exits with |
|---|---|---|
| `not_found` | a target matched nothing, or there is nothing to act on | 3 |
| `ambiguous` | a target matched several items; `error` lists them | 3 |
| `usage` | the request makes no sense | 2 |
| `no_client` | no app takes client actions | 1 |
| none | any other failure | 1 |

A request with an unknown `op` fails with no code.

### T-104 The arguments
status: proposed · traces: F-110, F-111, F-112, F-113 · source: code

`args` has these fields, each left out when empty:

| Field | Meaning |
|---|---|
| `session`, `window`, `block` | targets, as the user wrote them |
| `ctx_session`, `ctx_block` | the context (F-109) |
| `target` | a second target: an anchor, the other block, what to focus or wait for |
| `label`, `window_label` | a label; a new session's first window's label |
| `direction`, `ratio`, `amount`, `index`, `before`, `after` | placement; `ratio` from 0 to 1 |
| `no_focus`, `wrap`, `zoom` | `zoom` is `on`, `off`, or empty to toggle |
| `create` | a new block: `command`, `dir`, `shell`, `interactive`, `keep_open`, `label` |
| `data`, `keys`, `format`, `trim` | text to type, keys, capture's format |
| `all`, `model` | what a subscription wants |
| `client`, `action`, `params` | a client action |
| `kind`, `name`, `forget` | `hello`; `shutdown` |

The server resolves every target; `rex` sends them as typed.

### T-105 The requests of rex
status: proposed · traces: F-130, F-131, F-134, F-135, F-140, F-142, F-160, F-164, F-166, F-172, F-182 · source: code, tests

| `op` | Does | `data` |
|---|---|---|
| `model` | — | the model (T-107) |
| `list` | — | the blocks (T-108) |
| `clients` | — | `client_id`, `kind`, `name`, `connected_at`, `viewing_session_id` |
| `session.resolve`, `window.resolve`, `block.resolve` | applies F-110 to F-115 | the IDs |
| `session.new`, `window.new`, `block.split` | create | F-131, F-134, F-135 |
| `session.rename`, `session.move`, `session.kill` | | `session_id` |
| `window.close`, `window.rename`, `window.focus`, `window.move` | | `window_id` |
| `block.close`, `block.rename`, `block.focus`, `block.zoom`, `block.resize` | | `block_id` |
| `block.move`, `block.swap`, `focus` | | the blocks moved or focused |
| `send`, `sendKey`, `capture` | type, press keys, read | `capture`: `block_id`, `text` |
| `wait` | waits (F-166) | the target's ID, `exit_code` |
| `status.report`, `status.list` | T-110 | `block_id`, `records` |
| `subscribe` | T-106 | — |
| `client.do` | T-109 | the app's answer |
| `shutdown` | ends the server after 50 ms | — |

### T-106 Events
status: proposed · traces: F-169, F-170 · source: code, tests

`subscribe` replaces what the connection hears.
With `model`, it hears `{"type":"model","model":...}` at once and after
each change, and no other event.
With `session`, it hears every event of that session.
With `all`, it hears every event.
With neither, it hears the server's events and the sessions' events
without `block_event` and client events.
An event is `{"type", "time", "session_id", "payload"}`.
A slow connection loses events, never answers.

### T-107 The model
status: proposed · traces: F-130 · source: code, tests

The model is `{"revision", "sessions", "blocks"}`.
A session has `session_id`, `label`, `created`, `windows` and
`active_window_id`.
A window has `window_id`, `label`, `layout`, `focused_block_id`,
`zoomed_block_id` and `recent`.
A layout is a leaf `{"block_id"}`, or a split `{"split_id", "vertical",
"ratio", "a", "b"}`.
`vertical` puts `a` above `b`, else `a` is left of `b`; `ratio` is `a`'s
share.
`blocks` maps each block ID to `block_id`, `label`, `session_id`,
`window_id`, `shell`, `command`, `pid`, `cols`, `rows`, `keep_open`,
`exited`, `exit_code`, `created` and `restored`.

### T-108 A block's state
status: proposed · traces: F-130 · source: code, tests

`list` returns an object for each block, in layout order, with these
fields:

```
id label session_id window_id keep_open pid shell created program args
idle dir title lastOutput lastInput output bells exited exitCode
attached status cols rows
```

`program` is the foreground process's name; `idle` tells that the shell
is at its prompt.

### T-109 Client actions
status: proposed · traces: F-174, F-175, F-193 · source: code, tests

For `client.do`, the server sends the chosen app the event
`client_action`, with `action_id`, `name`, `params`, `ctx_session` and
`ctx_block`.
The app answers with the request `client.reply`: `action_id`, and
`result` or `error`.
The server then answers `client.do` with the result, or fails it with the
error.
It fails it after 30 seconds without an answer, or when the app goes.
`client.inspect` answers with the client record of the app that
`client.do` would choose for the same `client`, or fails as it would.

### T-110 A status report
status: proposed · traces: F-172, F-173 · source: code, rex-docs, tests

`status.report` takes the body of an OSC 7501 report in `data`:
`key=value` pairs joined by `:`.
`state` is needed; `id`, `kind`, `progress` and `app` are plain;
`title` and `msg` are base64, padding optional.
Keys are lower-case letters, at most 16.
A value holds only letters, digits and `_.,+/=-`.
A malformed pair is skipped, and the last of a key wins.
A body over 4096 bytes, a bad `id`, bad base64, or control characters in
text refuse the whole report.

| Field | Limit |
|---|---|
| `id` | path of up to 8 segments of `[A-Za-z0-9_.+-]`, 32 each, 128 in all |
| `app` | 32 characters of `[A-Za-z0-9_.+-]` |
| `title` | 192 bytes decoded |
| `msg` | 2048 bytes decoded |
| records | 256 a block; the oldest goes |

### T-111 IDs
status: proposed · traces: F-113, F-116 · source: code

An ID is its kind, `:`, and 12 random hexadecimal digits:
`session:0a1b2c3d4e5f`.
The kinds are `session`, `window`, `block`, `split` and `client`.
A short ID is `…` and the last six digits.
