# GoRex

A terminal for Linux, and Linux only, compatible with Superlogical's Rex: a
server keeps sessions of windows of terminal blocks, a MyGo GUI shows them,
and the `rex` CLI drives them. Go; the GUI is the main package, the server
and protocol are `internal/rex`.

## The spec

`spec/` describes the whole system in three layers, business, functional
and technical, with decisions beside them. Read `spec/README.md` before
changing behaviour. In short:

- Before changing code, find what it is meant to do:
  `go run ./tools/spec for FILE` and `go run ./tools/spec show ID`.
- Every change leaves the spec and the code agreeing when it merges.
  Behaviour that is new or changed goes into the spec, as `proposed`
  items; only the owner accepts, rejects or defers. Never invent a
  business reason: ask, or mark it as unknown.
- A test names the items it verifies: `// spec: F-12` right before it.
  Code that is not a test does not mention spec IDs.
- The `spec-propose` skill drafts a spec change.

## Checks

`mise run check` must pass: gofmt and shfmt, golangci-lint (.golangci.yml),
shellcheck, actionlint, tests with the race detector, and `spec check`.
In Claude Code, hooks format and lint each file as you edit it, and run
the check before you finish. `mise install`
installs the tools; `mise run format` formats.

## Observing it

The server and the CLI write wide events, one JSON line per request, event
and command, to `~/.config/gorex/events/`. Query them, with the spec and the
test results, with `go run ./tools/spec query "SQL"`; the `gorex-events`
skill lists the fields and useful queries. When something misbehaves, look
at the events before guessing; when a feature lacks the events to show it
works, add the fields.

Do not try a change on the user's running GoRex: run a server of your own
with `REX_SERVER= GOREX_DIR=$(mktemp -d)`: in a GoRex terminal, `rex` reads
`REX_SERVER` before `GOREX_DIR`.
