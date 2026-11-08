# Experiment: specify GoRex after the fact

GoRex was built without a spec, by agents, after Superlogical's Rex for
macOS. Its intent is in Rex's documentation and in the code; nobody can be
interviewed about it. Fresh agents, which know only the repository and
Rex's documentation, write the spec by `spec/README.md`. Then we see
whether it is usable: readable, true to the code, and, above all, whether
GoRex can be extended through it.

The owner skims and accepts the spec area by area, as a baseline, and
fills gaps where they matter. The real test comes after: new features go
through `spec-propose`, and we see if the spec helps or gets in the way.

## The pilot

One agent, one pass:

1. The business layer, small: who GoRex is for and why, from Rex's
   documentation and the repository (README, git history), with
   GoRex's own reasons (Rex on Linux) and constraints. The decisions the
   code and history show, with the options they rejected.
2. `spec/glossary.md`: the terms, one meaning each.
3. The CLI, as the first functional area.
4. `spec/FINDINGS.md` and `spec/QUESTIONS.md`.

The prompt is `docs/retro-spec-pilot.md`. The agent does not change code or
write tests: it tags the tests that exist, and reports the rest as gaps
and findings. Fixing the findings is a step of its own, afterwards.

Then the owner skims it: is it readable, is it true, is it too much? We
change `spec/README.md` where the pilot shows it is wrong, and specify the
next areas the same way:

| Area | Functional | Technical |
|---|---|---|
| CLI: commands, targets, output, exit codes, environment (pilot) | F-100… | T-100… |
| Model and layout: sessions, windows, blocks, splits, focus, zoom, restore | F-200… | T-200… |
| GUI: tabs, panes, palette, keymap, settings, notifications | F-300… | T-300… |
| Programs: shells, terminals, program status, agent skills and hooks, events | F-400… | T-400… |

## What we look at

- Readability: can the owner skim an area in minutes and know what GoRex
  does? Words per item, items per area, `spec check`'s writing warnings.
- Truth: does the spec say what the code does? Spot checks against the
  app and the events.
- Findings: competing behaviour, accidents, differences from Rex. A spec
  written after the fact is a review of the code.
- Coverage: functional items with tests; the gaps.
- Extension: the next feature (access from an Android phone) goes in
  through `spec-propose`. Does the spec help, or is it a mess to work in?
- Later, reimplementation: an agent with `spec/` and no code implements
  one area in Rust; the ported tests show what the spec did not say.
