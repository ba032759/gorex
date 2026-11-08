#!/usr/bin/env bash
# Stop: before the agent ends its turn, mise run check must pass, when anything
# changed since it last passed. On failure the agent is told and goes on
# (exit 2). After three blocks in a row it may stop, to not loop forever.
cd "${CLAUDE_PROJECT_DIR:-.}" || exit 0
mkdir -p .spec
state=$({
	git diff HEAD 2>/dev/null
	git ls-files --others --exclude-standard 2>/dev/null | while read -r f; do cat "$f" 2>/dev/null; done
} | sha256sum | cut -c1-16)
[ "$(cat .spec/last-pass 2>/dev/null)" = "$state" ] && exit 0
if out=$(mise run --quiet check 2>&1); then
	echo "$state" >.spec/last-pass
	echo 0 >.spec/stop-blocks
	exit 0
fi
n=$(($(cat .spec/stop-blocks 2>/dev/null || echo 0) + 1))
if [ "$n" -gt 3 ]; then
	echo 0 >.spec/stop-blocks
	echo "mise run check still fails after 3 attempts; stopping anyway. Tell the user what fails." >&2
	exit 0
fi
echo "$n" >.spec/stop-blocks
echo "mise run check fails; fix it before you finish (attempt $n of 3):" >&2
echo "$out" | tail -60 >&2
exit 2
