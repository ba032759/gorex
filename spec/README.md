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
name is the item's area unless it says `area:`. Beside them:

- `glossary.md`: the terms, one meaning each (see Writing).
- `FINDINGS.md`: problems found while specifying: competing behaviour,
  behaviour nobody would choose, what cannot be tested, differences from
  Rex. Each finding names the items and code it concerns.
- `QUESTIONS.md`: what only the owner can answer.

These three, and this file, hold no items.

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
- **source**: where the item comes from: `owner`, `rex-docs` (Superlogical's
  documentation of Rex), `repo` (the README, docs and git history), `code`,
  `tests`; several joined by `,`. An item
  only the code shows is a description of what is, not yet of what is
  wanted.
- Fields go one per line or on one line joined by `·`. Any other field
  (`rationale:`, `actor:`, `priority:`) is kept.

Items specified after the fact (the retro spec) are accepted area by area
as a baseline: the owner skims an area and accepts it whole, and corrects
what matters when it matters.

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

## Writing

Readability comes first. Simple technical English, after ASD-STE100:

- One word, one meaning. Use the terms of `glossary.md`, and only in their
  meaning there. A term the glossary does not have is added to it, or not
  used. Words it lists as `avoid:` are not used for that thing (`spec
  check` warns); a longer term that contains one, as "tab bar", is fine.
- Short sentences. One requirement per sentence.
- Active voice. Say who does it: the server, the app, `rex`, the user.
- A requirement says `SHALL` or `SHALL NOT`, or `WHEN … THEN`. In a
  requirement, not should, may, can, will, typically, usually, as needed:
  they blur what must hold. A description says what is, in plain words,
  and may use them.
- No filler: no "note that", "in order to", "it is important that".
- Code, commands, keys and names go in `backticks`.
- Say each thing once, at the level a reader needs. A functional item
  states a behaviour an observer can check; the full list of a command's
  flags is the command's help, and need not be copied.

`spec check` warns on what can mislead: vague words in a requirement,
avoided terms, unknown sources. Long sentences and long items are style
notes, shown with `spec check --style`: hints, not limits. `spec stats`
shows each area's size and reading time; that is the number to watch.

The glossary's entries are headings, with an optional `avoid:` line:

```markdown
### block
A terminal in a window, with its program: the leaf of a layout.
avoid: pane
```

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
go run ./tools/spec stats          items, words and reading time per area
go run ./tools/spec query "SQL"    DuckDB over the spec, the tests, their results and GoRex's events
```

`spec query` has these tables: `items`, `links`, `tests`, `problems`,
`test_results` (from the last `mise run test`: `go test -json` lines) and
`events` (GoRex's wide events; the `gorex-events` skill lists their fields).
