# Technical: what every command of rex shares

### T-140 The socket's path
status: accepted · traces: F-106 · source: code

The server listens on `server.sock` in its data directory.
When that path has 100 characters or more, too long for a Unix socket,
it listens on `gorex-UID-HASH.sock` in the system's temporary directory.
`UID` is the user's ID; `HASH` is the 32-bit FNV-1a hash of the long
path, in hexadecimal.
`rex`, without `REX_SERVER`, computes the same path from the data
directory.

### T-141 Marks and JSON of listings
status: accepted · traces: F-116 · source: code, tests

A table's first column holds a mark:

| Listing | Mark | Meaning |
|---|---|---|
| `rex ls`, `rex session ls` | `*` | the context session |
| `rex window ls`, `rex session inspect` | `*` | the active window |
| `rex block ls` | `@` | the context block |
| `rex block ls` | `*` | a focused block, not the context block |
| `rex list` | `*`, `>` | the active window, the focused block |

`--json` prints one JSON object, indented by two spaces, with full IDs.
`rex events --json` and `rex wait --json` print one object on one line.
T-124 lists the columns and fields of each command.

### T-142 A block's environment
status: accepted · traces: F-118 · source: code, tests

The server leaves out of its own environment what other terminals and
GoRex set: `TERM`, `COLORTERM`, `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`,
`TERM_SESSION_ID`, `TMUX`, `TMUX_PANE`, `STY`, `WINDOWID`, `COLUMNS`,
`LINES`, `SHLVL`, `OLDPWD`, `PWD`, `_`, and every variable that starts
with `ITERM_`, `GHOSTTY_`, `KITTY_`, `VSCODE_`, `WEZTERM_`,
`ALACRITTY_`, `GOREX_`, `MYGO_` or `REX_`. It then sets:

| Variable | Value |
|---|---|
| `REX_SESSION` | the block's session ID |
| `REX_BLOCK` | the block's ID |
| `REX_SERVER` | `unix:` and the socket's path (T-140) |
| `REX_BIN_DIR` | `bin` in the data directory |
| `GOREX_DIR` | the data directory |
| `PATH` | `REX_BIN_DIR`, then the server's `PATH` |
| `TERM`, `COLORTERM` | `xterm-256color`, `truecolor` |
| `TERM_PROGRAM`, `TERM_PROGRAM_VERSION` | `GoRex`, its version |
| `LANG` | `en_US.UTF-8`, when no `LANG`, `LC_ALL` or `LC_CTYPE` is set |

When it starts, the server makes `REX_BIN_DIR/rex` a symbolic link to
its own executable.

### T-143 What rex version prints
status: accepted · traces: F-123 · source: code

Without `--json`: `rex VERSION (GoRex, protocol N)`, and, when a server
answers, a second line `server pid PID, protocol N`.
With `--json`: `version`, `protocol`, `name` (`GoRex`), and, when a
server answers, `server`: `pid`, `protocol`, `started` and `exe`.
