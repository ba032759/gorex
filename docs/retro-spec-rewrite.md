You work in the GoRex repository at `~/repos/github.com/ba032759/gorex-retro-pilot`
(a git worktree; stay in it). GoRex is a terminal for Linux, compatible
with Superlogical's Rex for macOS.

Rewrite `spec/functional/cli-layout.md` so that it follows `spec/README.md`.
Read that file first, whole, and keep to it: its templates, above all
the functional one, and its writing rules. The present items were written
before the templates; they are your starting point, not your model.

Sources: the code (`cli.go`, `help.go`, `internal/rex/`), the tests, and
Rex's documentation at https://www.superlogical.com/rex/docs. Check what
an item says against the code before you keep it.

- Keep an item's ID when the item stays. When one item becomes several,
  the main part keeps the ID and the others take the next free IDs. An
  item that goes is `superseded` with `superseded-by:`; never delete one.
- What moves to the technical layer goes into `spec/technical/`, IDs from
  the next free T- number.
- Do not change code and do not write tests. Move a test's `// spec:` tag
  where the item it verifies now has another ID.
- `go run ./tools/spec check spec/functional/cli-layout.md` must pass
  (warnings fail it), and `mise run check` too. Commit once.

Report, in under 25 lines: `go run ./tools/spec stats` for the file before
and after; what you moved and where; the conflicts you recorded; and where
`spec/README.md` was unclear or did not fit what you had to say.
