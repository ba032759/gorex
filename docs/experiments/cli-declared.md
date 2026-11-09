# Experiment: declare rex's commands with kong

Written before the change, so that the result is judged by what we
expected, not by what we got.

## Question

`rex` parses its own flags (`cli.go`, about 70 lines of parser). A command
lives in five places: the command list, its handler's flag declarations,
the defaults in its logic, `help.go`, and completion. Does declaring the
commands with an established library (kong) make GoRex better for a user
and for the next person or agent who changes it, or does it only move code
so that our spec tools see it?

## Hypothesis

Better, if the declarations become the one place where a command is
defined: the help, the completion and `spec/reference/` follow from them,
and kinds of fault that come from copies (a flag missing from its help, two
checks of one value that disagree) cannot happen any more.

Only moved, if the declarations are as long as the old code, the help
stays hand-written beside them, and the faults remain.

## Measures

| Measure | Before (retro-pilot, 2026-11-09) | Better if |
|---|---|---|
| Flags missing from their command's help (`spec/reference/cli.md`) | 13, in 3 commands | 0, by construction |
| Findings that come from copies: 1 (`--ratio` ranges), 15 (help says less), 17 (flags taken and ignored), 29 (inspect takes no flags), 32 (no defaults in the reference) | 5 open | resolved by the structure, not by hand |
| Defaults in `spec/reference/cli.md` | none | all |
| The reference's probe: commands it cannot run safely (`noFlags`), timeout | 6, and a timeout | none: the reference reads the declarations |
| Lines of `cli.go` + `help.go` | 1991 + 178 = 2169 | equal or fewer |
| Size of the executable, stripped (`go build -ldflags=-s`) | 15.7 MB | not larger (D-12) |
| Places to edit to add a flag to a command | handler, help, (reference) | one |
| Behaviour other than the help's layout | — | unchanged: the tests pass; every change in the reference's diff and in any output is explained and in the spec |
| Rex compatibility (B-5): exit statuses, flags anywhere, `--`, several spellings | as now | as now |

Three branches, from `retro-pilot`: `cli-kong` (kong), `cli-stdlib` (one
declaration per command, Go's standard library only; added after kong's
run, 2026-11-09), and `retro-pilot` itself, as it is.

Phase 2, on each branch, by a fresh agent per change, the same prompt:

1. A local change: `rex split` and `rex run` take `-q`, `--quiet`: on
   success they print nothing.
2. A change across commands: every flag that takes a percentage
   (`--ratio` of `split`, `run` and `move`, `--amount` of `resize`) also
   takes a fraction (`0.3` is 30 %) and a trailing `%`. IF the value is
   outside the flag's range, THEN `rex` exits with 2, with one message
   that names the flag and its range. The ranges stay as they are
   (Finding 1 is the owner's).

Count for each: files and lines changed, the agent's tool calls, the
places a percentage is parsed afterwards, and whether the spec, the help
and the reference agree.

## How

A fresh agent on branch `cli-kong` (worktree `gorex-kong`, from
`retro-pilot`, where the spec is), with the official documentation of
kong only, its current release, and `spec-propose` for the spec change.
If kong does not fit a need of rex's command line, it says so and stops
rather than forcing it.

## Result

(after the run)
