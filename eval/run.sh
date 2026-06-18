#!/usr/bin/env bash
# Best-effort headless runner -- drive a full /ccr:review on <repo> via `claude -p`,
# then emit the SHIPPED findings as JSON (post --min-confidence: what the user sees).
#
# GATING RISK: this depends on `claude -p` dispatching the review's file-reviewer and
# reflector subagents to completion. If that does not work in your setup, run
# /ccr:review on the repo manually, then point recall.sh / consistency.sh at the
# review's .ccr/tmp/<run>/reflected.json via:
#   ccr-engine report --reflected <that> --format json --min-confidence 0.5
#
#   eval/run.sh <repo> [--from REF --to REF | --commit SHA]   (default: --from HEAD~1 --to HEAD)
set -euo pipefail
repo="${1:?usage: run.sh <repo> [review selectors...]}"; shift || true
sel="${*:---from HEAD~1 --to HEAD}"
min="${CCR_MIN_CONFIDENCE:-0.5}"
# A bare `claude -p` stalls on the workspace-trust / permission prompts, so the
# review never completes. bypassPermissions lets the reviewer's own read-only
# tools (Read/Grep/Glob + ccr-engine) run unattended -- ccr never executes the
# code under review, so this does not run target/PR code. Override if you prefer
# a narrower mode (e.g. CCR_REVIEW_PERM=default with a pre-trusted repo).
perm="${CCR_REVIEW_PERM:-bypassPermissions}"

command -v claude >/dev/null || { echo "claude CLI not found; run /ccr:review manually" >&2; exit 3; }
command -v ccr-engine >/dev/null || { echo "ccr-engine not on PATH; run 'make install'" >&2; exit 3; }

( cd "$repo" && claude -p "/ccr:review $sel" --permission-mode "$perm" >/dev/null 2>&1 ) || {
  echo "headless review failed: claude -p could not drive /ccr:review" >&2; exit 3; }

reflected="$(ls -t "$repo"/.ccr/tmp/*/reflected.json 2>/dev/null | head -1 || true)"
[ -n "$reflected" ] || { echo "no reflected.json under $repo/.ccr/tmp -- review did not complete" >&2; exit 3; }

ccr-engine report --reflected "$reflected" --format json --min-confidence "$min"
