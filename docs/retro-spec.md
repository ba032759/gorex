# Experiment: specify GoRex after the fact

GoRex was built without a spec. Fresh agents, which know only the
repository, write its spec from the code, by `spec/README.md`. Then we see
whether the spec is usable: can the owner review and accept it, can an
agent extend GoRex through it, can another implementation be built from it.

## Phase 1: the business layer, and the questions (one agent)

Prompt:

> Read spec/README.md, AGENTS.md, README.md, and skim the code, the CLI's
> help (help.go) and the tests. Draft spec/business/ and spec/decisions/
> for GoRex: who it is for, what they need, what constraints it lives
> under, and the decisions visible in the code and its history (git log),
> with the options they rejected. Everything is status: proposed. Where the
> reason is not in the repository, do not guess: write rationale: unknown,
> and add the question to spec/QUESTIONS.md, grouped by item, for an
> interview with the owner. Use IDs B-1… and D-1…. Run spec check.

Then the owner is interviewed on spec/QUESTIONS.md, and accepts, corrects
or rejects the business items. The rest builds on them.

## Phase 2: functional and technical layers (agents in parallel)

One agent per area, each in its own worktree, each with its own ID range
so that numbers do not collide:

| Agent | Area | Functional | Technical |
|---|---|---|---|
| 1 | CLI: commands, targets, output, exit codes, env | F-100… | T-100… |
| 2 | Model and layout: sessions, windows, blocks, splits, focus, zoom, restore | F-200… | T-200… |
| 3 | GUI: tabs, panes, palette, keymap, settings, notifications | F-300… | T-300… |
| 4 | Programs: shells, PTYs, program status (OSC 7501), agent skill and hooks, events | F-400… | T-400… |

Prompt:

> Read spec/README.md and spec/business/. Specify the area ‹area› of GoRex
> from its code and tests: functional items for what an observer outside
> sees, technical items for what another implementation would have to keep
> (protocols, formats, invariants). Trace each to the business items it
> serves; where none fits, add the question to spec/QUESTIONS.md. Use IDs
> from F-‹n›00 and T-‹n›00. Status: proposed. Mark the existing tests that
> verify an item with `// spec: ID`; list the items that have no test in
> your report, do not write tests yet. Run spec check until it has no
> errors.

## Phase 3: consolidate (one agent)

Merge the worktrees, resolve overlaps (one behaviour, one item), and report
`spec check --strict`, `spec matrix`, and the open questions.

## What we measure

- Size: items per layer, words per item. Too long to review is a failure.
- Review: how long the owner takes to accept a layer, and how much is
  corrected.
- Coverage: accepted functional items with tests; gaps found.
- Extension: the next feature (phone access) goes through spec-propose;
  does the spec help or get in the way?
- Reimplementation: an agent, with spec/ and no code, implements one area
  (the program status parser, target matching, the key encoder) in Rust;
  the Go tests' cases, ported, run against it. What fails is what the spec
  did not say.
