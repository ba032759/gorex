---
name: spec-propose
description: Draft a change to GoRex's spec (spec/) for the owner to approve - from the current diff, from what was explored in this session, or from an idea described in words. Use when behaviour was added or changed, before a pull request, when the user asks to "write this down as spec" or "propose a spec", or when spec check reports gaps.
---

# Propose a spec change

The spec is the owner's statement of intent. Your job is to draft, not to
decide: what you write is `proposed` until the owner accepts it. Read
`spec/README.md` first if you have not in this session, and its
Templates section always: every item you write copies the template of its
layer, slot for slot. Do not invent a shape of your own; two agents must
write the same need the same way.

## 1. Find what changed

- From a diff: `git diff` (and `git diff --staged`), and what the session
  tried and dropped: a rejected approach is a decision worth recording.
- From an idea: what the user said, in their words.
- Find the items it touches: `go run ./tools/spec for FILE...` for each
  changed file, `go run ./tools/spec ls`, `go run ./tools/spec show ID`.

## 2. Ask for the why

The business layer is the owner's. Do not invent needs, reasons or
priorities. If the change serves a business item that exists, trace to it.
If it needs a new one, ask the user, briefly, with your best guess to
correct:

> This adds X. I think the reason is Y (serves B-3?). Is that right, or why
> do you want it?

If they are not there to answer, write the item with `status: proposed`
and `rationale: unknown - ask the owner`.

## 3. Draft the items

- Edit the spec files in place, in the area the behaviour belongs to; a new
  area gets a new file. Take the next free number of the prefix:
  `go run ./tools/spec query "select max(cast(split_part(id,'-',2) as int)) from items where id like 'F-%'"`.
- Business, a need: `**Who.**`, `**Needs.**`, `**Why.**`, `**Fails if.**`.
  A limit the owner sets (systems, law, budget): `kind: constraint` with
  `**Constraint.**`, `**Why.**`, `**Fails if.**`. No format, protocol,
  size or technology in a business item: that is a decision. The Why is
  the owner's: `**Why.** Unknown.` and a question, never your guess.
- Decision: `**Context.**`, `**Options.**`, `**Choice.**`, `**Reason.**`,
  and `**Consequences.**` when something follows from it.
- Functional, one of three forms: a rule (`SHALL`, `WHEN … THEN`, one per
  item); a use case (`**Actor.**`, `**Goal.**`, numbered steps,
  `**Extensions.**`); a table of cases with a sentence that says what it
  holds. What an observer outside sees. Say how it is verified
  (`verify: test | event | manual`).
- Technical items for what a reimplementation must keep (a protocol, a file
  format, an invariant), not for how the code happens to be organised.
- Changed behaviour: edit the item, keep its ID; the diff shows the change.
  Behaviour that went: `status: superseded` with `superseded-by:`, or
  rejected with a rationale. Never delete an item, never renumber.
- An option considered and dropped: a `D-` item, `status: rejected`, with
  the reason.
- Write as little as says it. If an item says how, it is in the wrong
  layer.
- `go run ./tools/spec check FILE` after each file (the edit hook runs it
  too): it reports a missing slot, a vague requirement or an avoided term
  in what you just wrote.

## 4. Link the tests

Each functional item verified by tests gets `// spec: F-n` before the test
that verifies it. Write the test when there is none. Then:

```
go run ./tools/spec check
```

## 5. Hand it over

Tell the user, in a short list: the items you added or changed (ID, title,
one line), the questions you could not answer, and what they must decide
(accept, reject, defer). The status change is theirs.
