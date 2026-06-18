#!/usr/bin/env bash
# Recall scorer -- how many planted bugs in a fixture get flagged.
#   eval/recall.sh <fixture-dir>                  # run the review via run.sh (model, best-effort)
#   eval/recall.sh <fixture-dir> <findings.json>  # score a pre-captured findings JSON (model-free)
# A planted bug is "caught" when some finding shares its (file,line). Keyed on
# (file,line) only -- the model tags rule_id inconsistently.
# Exit 0 only if EVERY planted bug is caught -- usable as a regression tripwire.
# NOTE: planted-bug recall is a tripwire, NOT a real recall %.
set -euo pipefail

fixture="${1:?usage: recall.sh <fixture-dir> [findings.json]}"
expected="$fixture/expected.json"
[ -f "$expected" ] || { echo "no expected.json in $fixture" >&2; exit 2; }
here="$(cd "$(dirname "$0")" && pwd)"

if [ "${2:-}" ]; then
  findings="$2"
else
  # materialize before/ then after/ as a two-commit throwaway repo, then review it
  repo="$(mktemp -d)"; findings="$(mktemp)"
  trap 'rm -rf "$repo" "$findings"' EXIT
  cp -R "$fixture"/before/. "$repo"/
  git -C "$repo" init -q
  git -C "$repo" add -A >/dev/null
  git -C "$repo" -c user.email=e@e -c user.name=ccr commit -qm base
  rm -rf "${repo:?}"/*
  cp -R "$fixture"/after/. "$repo"/
  git -C "$repo" add -A >/dev/null
  git -C "$repo" -c user.email=e@e -c user.name=ccr commit -qm head
  "$here/run.sh" "$repo" --from HEAD~1 --to HEAD > "$findings"
fi

found="$(jq -r '.findings[]? | "\(.file):\(.line)"' "$findings" | sort -u)"
caught=0; total=0; missed=""
while IFS= read -r k; do
  [ -z "$k" ] && continue
  total=$((total + 1))
  if grep -qxF "$k" <<<"$found"; then caught=$((caught + 1)); else missed="$missed $k"; fi
done < <(jq -r '.expect[] | "\(.file):\(.line)"' "$expected" | sort -u)

echo "$(basename "$fixture"): caught $caught/$total planted${missed:+  (missed:$missed)}"
[ "$caught" -eq "$total" ]
