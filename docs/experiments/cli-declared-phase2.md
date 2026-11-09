You work in the GoRex repository at `{{WORKTREE}}`, a git worktree on
branch `{{BRANCH}}`. Stay in it; do not touch other worktrees or branches.
GoRex is a terminal for Linux, compatible with Superlogical's Rex for
macOS. Read `AGENTS.md` first.

Make this change, as a pull request to GoRex would: code, tests, help and
spec together. Use the `spec-propose` skill
(`.claude/skills/spec-propose/SKILL.md`) for the spec; what you write
there is `proposed`. `mise run check` passes when you finish. Test with
your own data directory (`GOREX_DIR=$(mktemp -d)`), never the user's
GoRex. Commit once.

The change:

{{CHANGE}}

Report, in under 15 lines: the files you changed; where in the code a
percentage is parsed after your change (file:line, if the change is about
percentages); anything in the code that made the change harder than it
should be.
