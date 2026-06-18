#!/usr/bin/env bash
# Self-check for the scorers -- no model, no network. The smallest thing that
# fails if the (file,line) matching or Jaccard math breaks. Run: bash eval/test_score.sh
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- recall: 2 planted; findings hit 1 + 1 unrelated -> caught 1/2 ---
mkdir -p "$tmp/fix"
printf '%s\n' '{ "expect": [ {"file":"a.go","line":10}, {"file":"b.go","line":20} ] }' >"$tmp/fix/expected.json"
printf '%s\n' '{ "findings": [ {"file":"a.go","line":10}, {"file":"c.go","line":99} ] }' >"$tmp/partial.json"
out="$("$here/recall.sh" "$tmp/fix" "$tmp/partial.json" || true)"
echo "$out"
grep -q "caught 1/2 planted" <<<"$out" || { echo "FAIL recall count: $out" >&2; exit 1; }
grep -q "b.go:20" <<<"$out"          || { echo "FAIL recall missed-list: $out" >&2; exit 1; }

# perfect recall -> exit 0
printf '%s\n' '{ "findings": [ {"file":"a.go","line":10}, {"file":"b.go","line":20} ] }' >"$tmp/perfect.json"
"$here/recall.sh" "$tmp/fix" "$tmp/perfect.json" >/dev/null || { echo "FAIL: full recall must exit 0" >&2; exit 1; }

# any miss -> non-zero (tripwire contract)
if "$here/recall.sh" "$tmp/fix" "$tmp/partial.json" >/dev/null; then
  echo "FAIL: partial recall must exit non-zero" >&2; exit 1
fi

# --- consistency: A,B share 1 key, 2 distinct -> Jaccard 1/3 = 0.333 ---
printf '%s\n' '{ "findings": [ {"file":"a.go","line":10}, {"file":"b.go","line":20} ] }' >"$tmp/a.json"
printf '%s\n' '{ "findings": [ {"file":"a.go","line":10}, {"file":"d.go","line":40} ] }' >"$tmp/b.json"
cout="$("$here/consistency.sh" "$tmp/a.json" "$tmp/b.json")"
echo "$cout"
grep -q "consistency: 0.333" <<<"$cout" || { echo "FAIL consistency math: $cout" >&2; exit 1; }

# identical sets -> 1.000 ; two empty sets -> 1.000
grep -q "consistency: 1.000" <<<"$("$here/consistency.sh" "$tmp/a.json" "$tmp/a.json")" || { echo "FAIL consistency identical" >&2; exit 1; }
printf '%s\n' '{ "findings": [] }' >"$tmp/empty.json"
grep -q "consistency: 1.000" <<<"$("$here/consistency.sh" "$tmp/empty.json" "$tmp/empty.json")" || { echo "FAIL consistency empty" >&2; exit 1; }

echo "OK: scorer self-check passed"
