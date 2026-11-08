You are specifying GoRex after the fact. GoRex is a terminal for Linux,
compatible with Superlogical's Rex for macOS, and built without a spec.
Write its spec by the rules of `spec/README.md`. Read that file first,
whole, and keep to it: the layers, the item format, the writing rules
(simple technical English, the glossary, SHALL).

Sources, and only these:

- Rex's documentation: https://www.superlogical.com/rex/docs and the
  pages it links to. Fetch what you need.
- This repository: README.md, AGENTS.md, the code, the CLI's help
  (`help.go`), the tests, and `git log`.

Do not change code, and do not write tests. This pass describes what is.

## What to write

1. `spec/business/`: who GoRex is for, what they need, why, and the
   constraints it lives under. Few items, 5 to 10: needs, not features.
   Mark each with its `source:`. Where a reason is in neither source,
   write `rationale: unknown` and add the question to `spec/QUESTIONS.md`.
2. `spec/decisions/`: the decisions the code and its history show (for
   example: the server owns the layout; no Lua; compatibility with Rex),
   with the options rejected and why, as far as the sources tell.
3. `spec/glossary.md`: every term the spec uses for a part of GoRex
   (session, window, block, client, target, …), one meaning each. Where
   Rex, the code and the GUI use different words for one thing, choose
   one, list the others as `avoid:`, and record the difference as a
   finding.
4. `spec/functional/cli.md` (split it into several files if it is more
   than one area): what the `rex` command does, as seen from outside: its
   commands, how it finds targets, its output, its exit statuses, the
   environment it reads and sets. IDs from F-100. Restate the behaviour;
   do not refer to Rex's documentation for it. Where GoRex differs from
   Rex, say what GoRex does, and record the difference as a finding.
5. `spec/technical/`: only what another implementation of the CLI would
   have to keep (the protocol to the server, file formats). IDs from
   T-100. Leave out how the code happens to be organised.
6. Tag the tests that verify an item: `// spec: F-1xx` right before the
   test function or `t.Run`. Only where the test really verifies it.
7. `spec/FINDINGS.md`: what you found while specifying. Each finding: a
   heading `## Finding N: title`, the kind (competing behaviour,
   accidental behaviour, untestable, difference from Rex, inconsistent
   terms, other), the items and the code it concerns (`file:line`), and
   what you saw. Competing behaviour is the most valuable kind: two parts
   of GoRex doing one thing differently.
8. `spec/QUESTIONS.md`: what only the owner can answer, grouped by item.

Every item is `status: proposed`. Business items trace to nothing;
functional items to business items; technical items to functional ones.

## How to work

- Read before writing: the CLI's code (`cli.go`, `help.go`), the server's
  handling of its requests (`internal/rex/`), and the tests.
- `go run ./tools/spec check` after each file. No errors when you finish;
  read the warnings and fix them (vague requirements, avoided terms,
  sources); look at `spec check --style` and `spec stats` for what reads long. If a rule of `spec/README.md` does not work for
  what you must say, do not break it quietly: record it as a finding of
  kind "spec rules".
- `mise run check` must pass at the end.
- Commit your work in this worktree when done, in one commit.

## When you finish

Report, in under 30 lines: the numbers (`spec check` and
`go run ./tools/spec matrix`), the five most important findings, the
questions, and what was hard about the rules.
