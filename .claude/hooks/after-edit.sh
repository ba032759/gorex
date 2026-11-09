#!/usr/bin/env bash
# PostToolUse on Edit/Write: format the file and lint it, by its kind; check
# the spec after a spec file changed. Problems go back to the agent (exit 2).
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
file=$(python3 -c 'import sys,json; print(json.load(sys.stdin).get("tool_input",{}).get("file_path",""))' 2>/dev/null)
[ -n "$file" ] && [ -f "$file" ] || exit 0
# The tools of mise.toml, also when the agent's shell did not activate it.
command -v mise >/dev/null && eval "$(mise env -s bash 2>/dev/null)"

fail() {
	echo "$1 (fix it before going on):" >&2
	echo "$2" | head -40 >&2
	exit 2
}

case "$file" in
*.go)
	out=$(gofmt -w "$file" 2>&1) || fail "gofmt failed" "$out"
	dir=$(realpath --relative-to=. "$(dirname "$file")")
	out=$(golangci-lint run "./$dir" 2>&1) || fail "golangci-lint found problems" "$out"
	;;
*.sh)
	# shellcheck disable=SC2086 # the flags are words
	out=$(shfmt $SHFMT_FLAGS -w "$file" 2>&1) || fail "shfmt failed" "$out"
	out=$(shellcheck "$file" 2>&1) || fail "shellcheck found problems" "$out"
	;;
*/.github/workflows/*.yml | */.github/workflows/*.yaml)
	out=$(actionlint "$file" 2>&1) || fail "actionlint found problems" "$out"
	;;
*/spec/*.md)
	out=$(go run ./tools/spec check -q "$file" 2>&1) || fail "spec check found problems in $file (the templates and writing rules are in spec/README.md)" "$out"
	;;
esac
exit 0
