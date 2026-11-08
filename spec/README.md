# How GoRex is specified

This directory describes the whole of GoRex: why it exists, what it does,
and how it is built, in three layers that follow the V model of GAMP 5.
The spec is the product's memory of intent. Code says what the system
does; the spec says what it is meant to do, and why, and tests and events
show that the two agree.

The spec is not meant to regenerate the code identically. It is meant to
hold enough intent that another implementation, by another person or
agent, in another language, would do the same things for the same
reasons.

## The layers

| Layer | Prefix | Says | Written by | Verified by |
|---|---|---|---|---|
| Business | `B-` | why: needs, goals, constraints, who it is for | the owner (an agent may draft) | the owner's acceptance, by hand |
| Decisions | `D-` | what was chosen, what was rejected, and why | whoever decides; the owner accepts | — |
| Functional | `F-` | what the system does, seen from outside | the owner and agents | tests, events, or a manual check |
| Technical | `T-` | how it is built: architecture, protocols, formats | agents | unit and integration tests |

Files: `business/`, `decisions/`, `functional/`, `technical/`, one markdown
file per area (`functional/layout.md`, `technical/protocol.md`). The file
name is the item's area unless it says `area:`.

Each layer traces up: a functional item names the business items it
serves, a technical item the functional ones. Nothing traces down; the
tools find what traces to an item.

## Items

An item is a heading with an ID, then its fields, then its text:

```markdown
### F-12 Answer a blocked session from the phone
status: accepted · traces: B-3 · verify: test

WHEN a session in any window is blocked on a permission prompt
THEN the phone SHALL show it within 5 seconds, with the prompt's text.
```

- **ID**: `B-`, `D-`, `F-` or `T-` and a number, unique in the spec, never
  reused, never renumbered. A removed item stays, with its status.
- **status**: `proposed`, `accepted`, `rejected`, `deferred`, `superseded`
  (with `superseded-by: F-20`). Agents write `proposed`; only the owner
  accepts, rejects or defers. Rejected and deferred items stay in the spec
  with a `rationale:`, so the idea is not proposed again unknowingly.
- **traces**: the IDs of the items above it.
- **verify** (functional): `test` (the default), `event` (shown by the wide
  events in real use), or `manual` (the owner checks it by hand; say how).
- Fields go one per line or on one line joined by `·`. Any other field
  (`rationale:`, `actor:`, `priority:`) is kept.

## How to write each layer

Write intent, not implementation, and as little as says it. An item a
reader can misread is too short; one that says how is in the wrong layer.

**Business.** Prose. Who has the need, what they want to get done, why it
matters, what would make it fail. Constraints (a one-person project, runs
on Linux and Android) are business items too.

**Decisions.** In the shape of an architecture decision record: the
context, the options considered, the choice, the reason, the consequences.
A rejected idea is a decision with `status: rejected`.

**Functional.** Pick the form that fits what is described:

- *Use cases* for interaction (Cockburn's form): actor, goal, the main
  success scenario as numbered steps, then extensions (`3a. The phone is
  offline: …`). Extensions are where most tests come from.
- *Rules* for exact behaviour: one requirement per item, `SHALL` for what
  must hold, `WHEN … THEN …` for a scenario (EARS style).
- *Tables* for exhaustive cases (exit codes, key names).

Say what an observer outside the system sees: the CLI's output and exit
status, what is on screen, the protocol's messages, what the events say.

**Technical.** Free form, kept short: components, protocols, file formats,
the event schema, the invariants a reimplementation must keep. Written by
agents for agents.

## Verification: tests name what they verify

A test says which items it verifies with a comment right before it, in any
language:

```go
// spec: F-12, F-14
func TestPhoneShowsBlockedSession(t *testing.T) {
```

```rust
// spec: F-12
#[test]
fn phone_shows_blocked_session() {
```

`spec check` finds them. Code that is not a test does not mention spec
IDs: comments in code drift; tests fail.

## The rule for every change

A change, a pull request, leaves the spec and the code agreeing when it
merges. It does not matter which came first: specify and then build, or
build until it feels right and then write down what it does. What matters
is that at the merge:

1. every behaviour the change adds or changes is in the spec, and the spec
   items it touched that the owner must accept say `proposed` until they
   are accepted;
2. the tests that verify the changed items say so (`spec:`);
3. `mise run check` passes.

A spec change shows in the pull request's diff, beside the code: that diff
is the change request, its review the decision. Two changes to the same
item conflict in git, as they should.

The `spec-propose` skill drafts the spec change from a diff, a session or
an idea.

## Tools

```
go run ./tools/spec check          errors, gaps (an accepted item with no test), warnings
go run ./tools/spec ls -l F -s proposed
go run ./tools/spec show F-12      an item, its traces both ways, its tests
go run ./tools/spec for FILE       the items the tests of FILE's package verify
go run ./tools/spec matrix         the traceability matrix, business to tests
go run ./tools/spec query "SQL"    DuckDB over the spec, the tests, their results and GoRex's events
```

`spec query` has these tables: `items`, `links`, `tests`, `problems`,
`test_results` (from the last `mise run test`: `go test -json` lines) and
`events` (GoRex's wide events; the `gorex-events` skill lists their fields).
