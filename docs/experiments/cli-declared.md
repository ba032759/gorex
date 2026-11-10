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

| Measure                                                                                                                                                                 | Before (retro-pilot, 2026-11-09) | Better if                                                                                                      |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Flags missing from their command's help (`spec/reference/cli.md`)                                                                                                       | 13, in 3 commands                | 0, by construction                                                                                             |
| Findings that come from copies: 1 (`--ratio` ranges), 15 (help says less), 17 (flags taken and ignored), 29 (inspect takes no flags), 32 (no defaults in the reference) | 5 open                           | resolved by the structure, not by hand                                                                         |
| Defaults in `spec/reference/cli.md`                                                                                                                                     | none                             | all                                                                                                            |
| The reference's probe: commands it cannot run safely (`noFlags`), timeout                                                                                               | 6, and a timeout                 | none: the reference reads the declarations                                                                     |
| Lines of `cli.go` + `help.go`                                                                                                                                           | 1991 + 178 = 2169                | equal or fewer                                                                                                 |
| Size of the executable, stripped (`go build -ldflags=-s`)                                                                                                               | 15.7 MB                          | not larger (D-12)                                                                                              |
| Places to edit to add a flag to a command                                                                                                                               | handler, help, (reference)       | one                                                                                                            |
| Behaviour other than the help's layout                                                                                                                                  | —                                | unchanged: the tests pass; every change in the reference's diff and in any output is explained and in the spec |
| Rex compatibility (B-5): exit statuses, flags anywhere, `--`, several spellings                                                                                         | as now                           | as now                                                                                                         |

Four branches, from `retro-pilot`: `cli-kong` (kong), `cli-stdlib` (one
declaration per command, Go's standard library only), `cli-urfave`
(urfave/cli v3; both added after kong's run, 2026-11-09), and
`retro-pilot` itself, as it is.

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

## Result (2026-11-09)

### Phase 1: the conversion

|                           | kong v1.17.0                                                                           | stdlib (own parser)                                                     | urfave/cli v3.14.0                                               |
| ------------------------- | -------------------------------------------------------------------------------------- | ----------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Converted                 | yes, 54 commands                                                                       | yes, 54 commands                                                        | no: stopped at the fit check                                     |
| Flags missing from help   | 13 → 0 (tested)                                                                        | 13 → 0 (tested)                                                         | —                                                                |
| Defaults in the reference | all                                                                                    | all                                                                     | —                                                                |
| Probe, `noFlags`, timeout | gone                                                                                   | gone (10 commands declared as reading their own arguments)              | —                                                                |
| Lines, CLI files          | 2169 → 2272 (+103)                                                                     | 2169 → 2685 (+516, `flags.go` 360)                                      | —                                                                |
| Stripped size             | 15.7 → 18.9 MB (+20 %)                                                                 | 15.7 → 15.8 MB (+0.8 %)                                                 | —                                                                |
| New dependencies          | kong                                                                                   | none                                                                    | —                                                                |
| Behaviour changes         | 21 of 175 command lines (stricter refusals, kong's messages, `--help` after a command) | 4 exit statuses at edges, of 213 command lines; server output identical | —                                                                |
| Rex compatibility         | two gaps: a flag inside a list, `-5` as an argument                                    | kept                                                                    | `--` merged into the arguments; `-5` stops flag parsing silently |

urfave/cli: the `-5` case decides it. `rex send -5 -b logs` would type all
three words into the user's own terminal with exit 0; kong refuses the
same line with 2. The `--` case alone was solvable as kong's branch solved
it (cut what follows `--` before parsing): the urfave agent called that
workaround a hack, the kong agent used it. Two agents judged one
workaround differently.

Both conversions found that Finding 18 was wrong; the stdlib branch found
two more accidents (`--autostart=1` means off; `capture --format`).

### Phase 2: two changes on each branch (one run each)

|                                                    | old                                                            | kong                             | stdlib                                      |
| -------------------------------------------------- | -------------------------------------------------------------- | -------------------------------- | ------------------------------------------- |
| `-q`: code lines (+/−), tool calls, minutes        | 7/4, 18, 3.3                                                   | 10/4, 24, 5.4                    | 9/5, 21, 3.9                                |
| `-q`: how `-q --json` ends                         | prints nothing                                                 | refused, exit 2, asked the owner | prints nothing                              |
| Percentages: code lines (+/−), tool calls, minutes | 48/14, 26, 5.1                                                 | 49/17, 26, 5.6                   | 25/11, 21, 4.8                              |
| Places a percentage is parsed, after               | 1 (the agent wrote the helper)                                 | 1 (plus range tags)              | 1 (was already 1)                           |
| The 0.5 question (half a percent, or 50 %)         | 50 %, marked `source: owner` (wrong: the owner did not decide) | 50 %, noted in Finding 1         | 50 %, D-14 "Reason: unknown, ask the owner" |

All six: `mise run check` passes, `spec check` has no errors, the
reference agrees (its test).

### What we learned

- The structure helps where copies drift: on every converted branch a flag
  can no longer miss its help, defaults are in the reference, and a
  percentage is parsed once. The cross-cutting change cost the stdlib
  branch half the code of the others.
- A library is not what brings it: one declaration per command does.
  kong costs 20 % of the binary and two compatibility gaps; urfave/cli
  does not fit; the standard library keeps size and compatibility at the
  price of 360 lines of parser.
- On the old code, the cross-cutting change made the agent write the
  shared helper itself: the refactor happens anyway, one change at a time,
  with the help still kept by hand.
- One run per cell: the differences in tool calls and minutes are within
  noise (the agents ran in parallel and waited on each other's lint).
- Agents differ more than architectures. One sentence ("print nothing";
  "also takes a fraction") gave two readings of `-q --json`, and three
  ways to record the 0.5 question, one of them a wrong attribution. The
  change request is the weakest part of the chain: it should come as a
  proposed spec item, with its open questions, before an agent builds it.

### Decided (2026-11-09)

The owner chose the standard library: `exp/cli-stdlib` is merged into
`retro-pilot` (D-13, accepted). The rest was decided by Rex's reference
(`docs/reference/cli.md`) and B-5: one `--ratio` range, 1 to 99; no
fractions (D-14, rejected); no `--quiet` for split and run, since Rex's
`-q` means IDs only (D-15, rejected); `rex client inspect` as Rex's, with
the server's matching (Findings 7, 29); flags taken for Rex stay, marked
ignored (Finding 17); the server's ratio clamps are Finding 35, deferred.

The branches are kept as tags: `exp/cli-kong`, `exp/cli-stdlib`, and
`exp/p2-{old,kong,stdlib}-{quiet,pct}`. `cli-urfave` had no commits.
