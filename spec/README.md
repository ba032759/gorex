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

| Layer      | Prefix | Says                                              | Written by                         | Verified by                      |
| ---------- | ------ | ------------------------------------------------- | ---------------------------------- | -------------------------------- |
| Business   | `B-`   | why: needs, goals, constraints, who it is for     | the owner (an agent may draft)     | the owner's acceptance, by hand  |
| Decisions  | `D-`   | what was chosen, what was rejected, and why       | whoever decides; the owner accepts | —                                |
| Functional | `F-`   | what the system does, seen from outside           | the owner and agents               | tests, events, or a manual check |
| Technical  | `T-`   | how it is built: architecture, protocols, formats | agents                             | unit and integration tests       |

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

WHEN a session in any window is blocked on a permission prompt, the
phone SHALL show it, with the prompt's text.
```

- **ID**: `B-`, `D-`, `F-` or `T-` and a number, unique in the spec, never
  reused, never renumbered. A removed item stays, with its status.
- **status**: `proposed`, `accepted`, `rejected`, `deferred`, `superseded`
  (with `superseded-by: F-20`, or several: `F-20, F-21`). Agents write `proposed`; only the owner
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
- **kind**: in business, `need` (the default) or `constraint`; in
  functional, `function` (the default), `performance`, `security`,
  `usability` or `operational`.
- **conflicts**: the items this one contradicts, when both describe what
  GoRex does (competing behaviour). On one of the two is enough; `spec
  show` shows it on both. Two accepted items in conflict are a gap: the
  owner resolves it.
- Fields go one per line or on one line joined by `·`. Any other field
  (`rationale:`, `actor:`, `priority:`) is kept.

Items specified after the fact (the retro spec) are accepted area by area
as a baseline: the owner skims an area and accepts it whole, and corrects
what matters when it matters.

## Templates

Every item of a layer has the same shape, whoever writes it, person or
agent, so that the spec reads as one document and the tools can check it.
A slot is a paragraph that starts with its label in bold. `spec check`
reports an item without its slots as an error; rejected and superseded
items keep the shape they had.

Write intent, not implementation, and as little as says it. An item a
reader can misread is too short; one that says how is in the wrong layer.

### Business: needs and constraints (after Volere)

A need says who needs what, and why, and how you know that it is not met.
Prose, no `SHALL`: the functional items that trace to it make it exact.

```markdown
### B-3 Scripts drive terminals

status: proposed · source: rex-docs, code

**Who.** A user who automates the work of the terminal.
**Needs.** To create sessions and terminals from a shell script, type into
them, read their screens, wait for their programs and react to events.
**Why.** Repeated work then runs without the user at the keyboard.
**Fails if.** A script must read text made for people, or cannot tell a
missing target from a failed command.
```

- **Who**: a role (a user who…, an agent, the owner), not a person.
- **Needs**: what they want to get done, in their words, not the feature
  that does it.
- **Why**: the reason. When no source gives it: `**Why.** Unknown.`, and
  a question in `QUESTIONS.md`. Never a guess.
- **Fails if**: what the owner would see if the need is not met: the fit
  criterion. Concrete enough to check by hand.

A constraint is a limit the owner sets, not a need of a user: the systems
it runs on, the law, the budget. It says `kind: constraint`:

```markdown
### B-7 Linux first

status: proposed · kind: constraint · source: repo

**Constraint.** GoRex runs on Linux, and also on macOS and Windows.
**Why.** The owner works on Linux, where Rex has no app.
**Fails if.** A feature works on macOS and not on Linux.
```

A business item names no format, protocol, size, library or technology:
JSON, sockets, 15 MB, no web view. Those are choices: decisions or
technical items. Background on the project (who builds it, how) is not an
item; it goes at the top of the business file, as plain text.

### Decisions (an architecture decision record)

```markdown
### D-5 A local server only

status: proposed · traces: B-6 · source: code

**Context.** What forces a choice.
**Options.** (a) … (b) …, each in a sentence.
**Choice.** (b).
**Reason.** Why (b), and why not (a). `Unknown.` when no source says.
**Consequences.** What follows from it, and when to look at it again.
```

`Consequences` is optional. An idea rejected outright is a decision with
`status: rejected` and a `rationale:`.

### Functional: one item, one use case (after Volere and EARS)

A functional item is one *product use case*: what GoRex does in answer to
one event from outside, one thing a user, a script or an agent sets off
and would name: "create a session", "split a block", "show a blocked
session on the phone". The title names it. Its rules are its fit
criteria: each one a sentence a test, an event or a person can check.

- One event per item. A second trigger is a second item; a variant of the
  same one (a flag, an error) is a rule of the item. One event stays one
  item whatever it acts on: renaming a session, a window or a block is
  one item.
- A rule is what the owner would want to be asked about before it
  changes: what happens, what is refused, what the user would miss or be
  surprised by. A flag that only sets a value (a label, a ratio, a
  directory) is not a rule; the reference lists it, the tests pin it
  down. A default is a rule only when the owner would be asked about it
  (the directory a new block starts in), else it is the help's. Usually
  five rules or fewer.
- Output formats (the columns of a table, the fields of `--json`, a
  file's layout) are technical items. The functional item says what the
  output tells.
- What several items share (the direction words, the options of a new
  block) is an item of its own, in a table, that the others name.
- A behaviour that contradicts another item gets `conflicts:`, until the
  owner resolves it.

Every rule is one sentence in one of the five EARS patterns. The
conditions come first, each closed by a comma; then the subject; then
`SHALL` or `SHALL NOT`:

| Pattern  | For                               | Form                     |
| -------- | --------------------------------- | ------------------------ |
| always   | what always holds                 | The X SHALL …            |
| event    | an event                          | WHEN …, the X SHALL …    |
| state    | while a state lasts               | WHILE …, the X SHALL …   |
| unwanted | an error, a fault, a misuse       | IF …, THEN the X SHALL … |
| feature  | where a feature or system applies | WHERE …, the X SHALL …   |

Patterns combine: `WHILE a block is zoomed, WHEN it is split, the app
SHALL …`. Errors take IF … THEN, not WHEN: they are where tests are most
often missing. `spec check` warns on a rule that fits no pattern.

```markdown
### F-131 Create a session

status: proposed · traces: B-3 · source: code, rex-docs, tests

`rex new` SHALL create a session, after the others, with one window of
one block.
The block SHALL start in the directory that `rex` runs in.
WHEN a command follows `--`, the block SHALL run it instead of the shell.
`rex new` SHALL print the new session's ID.
IF two labels are given, THEN `rex` SHALL exit with 2.
```

Three forms hold the rules:

- *Rules*, as above, for behaviour stated exactly.
- A *use case*, for an interaction of several steps (Cockburn's form): the
  actor, the goal, the main success scenario as numbered steps, then the
  extensions, where most tests come from. Rules may follow.

  ```markdown
  ### F-310 Change session from the keyboard

  status: proposed · traces: B-1 · verify: manual

  **Actor.** The user, in the app.
  **Goal.** Work in another session.

  1. The user presses `Ctrl+Shift+S`.
  2. The app shows the sessions, the current one marked.
  3. The user picks one; the app shows its windows.

  **Extensions.** 3a. The user presses `Escape`: nothing changes.
  ```

- A *table*, for exhaustive cases (exit statuses, key names), with a
  sentence that says what the table holds.

A quality GoRex must have, not a function, is an item of its own with a
`kind:` (Volere's non-functional requirements), in the same form:

```markdown
### F-420 Only the user's clients reach the server

status: proposed · traces: B-6 · kind: security · verify: test

IF a process of another user connects to the server's socket, THEN the
server SHALL refuse it.
```

Say what an observer outside the system sees: the CLI's output and exit
status, what is on screen, the protocol's messages, what the events say.

### Technical

Free form, kept short: components, protocols, file formats, the event
schema, the invariants a reimplementation must keep. Written by agents for
agents.

## Writing

Readability comes first. Simple technical English, after ASD-STE100:

- One word, one meaning. Use the terms of `glossary.md`, and only in their
  meaning there. A term the glossary does not have is added to it, or not
  used. Words it lists as `avoid:` are not used for that thing (`spec
  check` warns); a longer term that contains one, as "tab bar", is fine.
- Short sentences. One requirement per sentence.
- Active voice. Say who does it: the server, the app, `rex`, the user.
- A requirement says `SHALL` or `SHALL NOT`, in an EARS pattern
  (Templates). In a requirement, not should, may, can, will, typically, usually, as needed:
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

## The reference: made from the code

`spec/reference/` holds what the code has, listed by the code: every
command of `rex` and every flag it takes (`reference/cli.md`). Nobody
writes it: `mise run spec-ref` makes it, and a test fails when it
differs from the code, so it cannot drift. A changed flag or default
shows in the diff of a pull request, without a spec item to change.

The functional items say what the commands are for; the reference says
what they take. `spec check` reports a command of the reference that no
functional item names (`rex split` in its text) as a gap: code without
a stated intent. Write an item for it, or remove the command.

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
go run ./tools/spec check FILE     only FILE's problems; warnings fail too (the edit hook runs this)
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
