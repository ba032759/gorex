# Technical: the files of the data directory that rex uses

### T-120 The data directory
status: accepted · traces: F-106, F-118, F-183, F-430 · source: code

| File | What |
|---|---|
| `server.sock` | the socket (F-106) |
| `server.lock` | locked by the server that runs; one server for each directory |
| `server.log` | the server's standard output and error, appended |
| `state.json` | the saved model, and each block's directory, size, command and label |
| `settings.json`, `keymap.json` | the app's settings and keys (T-122) |
| `bin/rex` | a link to the server's executable |
| `events/YYYY-MM-DD.jsonl` | the wide events (T-121) |

The directory and its files are for the user alone: `0700` and `0600`.

### T-121 Wide events
status: accepted · traces: F-121 · source: code, tests

Each wide event is one JSON object on one line, appended with one write
to the file of its UTC day.
The server and every `rex` append to the same file.
Files older than 30 days are removed.
Every line has `ts` (UTC, microseconds, `Z`), `kind`, `version`,
`protocol`, `pid`, `os` and `arch`; `dropped_before` counts lines lost.
A line of kind `cli` adds `command` (the declared command's names),
`flags` (the declared flags given, by their names), `n_unknown_flags`,
`n_args`, `exit_code`, `ok`, `duration_ms`, `in_rex`, `error` (`usage`
for a usage error, whose message quotes the command line), `rex_session`,
`rex_block`, `client` and `agent`.
The `gorex-events` skill describes the other kinds.

### T-122 keymap.json
status: accepted · traces: F-178, F-179 · source: code, rex-docs

`keymap.json` is `{"preset": NAME, "bindings": [...]}`.
A binding is `{"key": KEYS, "action": NAME, "args": {...}}`.
`KEYS` is a key in the syntax of F-162, or several joined by `>` as a
sequence.
The action `none` unbinds the key.
Without `preset`, it is `macos` when `settings.json` has
`"macosKeybindings": true`, else `linux`.
A preset other than `linux`, `macos` or `none` is an error.

### T-123 Starting the server
status: accepted · traces: F-107, F-183 · source: code

A client starts a server by running its own executable with the one
argument `-server`, detached, with standard output and error appended to
`server.log`.
It then connects every 30 ms, for up to the client's timeout: `--timeout`
for `rex`, 15 seconds for the app.
A server that cannot lock `server.lock` exits at once.
