#!/usr/bin/env bash
# Consistency (stability) scorer -- Jaccard of the (file,line) finding sets from
# two reviews of the SAME input. No fixture, no expected.json: run /ccr:review on
# any real diff twice, capture each shipped findings JSON, compare.
#   eval/consistency.sh <findingsA.json> <findingsB.json>
# Measures STABILITY, not correctness -- a reliably-wrong reviewer scores 1.000.
# Two runs is one noisy sample.
set -euo pipefail
a="${1:?usage: consistency.sh <findingsA.json> <findingsB.json>}"
b="${2:?usage: consistency.sh <findingsA.json> <findingsB.json>}"

keys() { jq -r '.findings[]? | "\(.file):\(.line)"' "$1" | sort -u; }
ka="$(keys "$a")"; kb="$(keys "$b")"

inter="$(comm -12 <(printf '%s\n' "$ka") <(printf '%s\n' "$kb") | grep -c . || true)"
union="$(printf '%s\n%s\n' "$ka" "$kb" | sort -u | grep -c . || true)"

if [ "$union" -eq 0 ]; then
  echo "consistency: 1.000 (both runs found 0 findings)"
  exit 0
fi
jac="$(awk "BEGIN{printf \"%.3f\", $inter/$union}")"
echo "consistency: $jac  (|A∩B|=$inter |A∪B|=$union)"
only_a="$(comm -23 <(printf '%s\n' "$ka") <(printf '%s\n' "$kb") | grep . || true)"
only_b="$(comm -13 <(printf '%s\n' "$ka") <(printf '%s\n' "$kb") | grep . || true)"
[ -n "$only_a" ] && printf 'only in A:\n%s\n' "$only_a"
[ -n "$only_b" ] && printf 'only in B:\n%s\n' "$only_b"
true
