---
description: Hybrid code review of the current changes (or a GitLab MR / GitHub PR URL) — line-anchored findings with suggested fix diffs. Does not modify source files.
argument-hint: "[--from REF --to REF | --commit SHA | <gitlab-mr-url> | <github-pr-url>]"
allowed-tools: Bash, Read, Grep, Glob, Task
---

Run a code review using the `ccr-engine` binary for the deterministic steps and the `file-reviewer` / `reflector` subagents for judgment. Determinism lives in the engine; you are the orchestrator gluing the steps together.

**Prerequisite:** `ccr-engine` must be on PATH (`make install` from the ccr repo). For GitLab MR mode, `glab` must be installed and authenticated; for GitHub PR mode, `gh` must be installed and authenticated.

Arguments: `$ARGUMENTS`

Pick a short run id `RUN` (e.g. a timestamp like `20260605-104500`). Then set an **absolute** run directory and create it:

```
RUNDIR="$(pwd)/.ccr/tmp/$RUN"
mkdir -p "$RUNDIR/findings"
```

Use absolute paths everywhere below. Subagents do **not** reliably inherit this session's working directory, so every path you hand a subagent (especially `out_path`) must be absolute or it will write to the wrong place and the review will silently find nothing. Add `.ccr/` to the repo's `.gitignore`; these are temp files, not source edits.

### Step 0 — Resolve input mode
- **If `$ARGUMENTS` contains a GitLab MR URL** (matches `/-/merge_requests/`):
  1. Run `ccr-engine mr-prep --url <that-url>`.
  2. Parse the JSON output. Capture `repo`, `workdir`, `base_sha`, `head_sha`.
  3. Plan args = `--repo <workdir> --from <base_sha> --to <head_sha>`. Remove the worktree at the end.
- **Else if `$ARGUMENTS` contains a GitHub PR URL** (matches `/pull/`):
  1. Run `ccr-engine pr-prep --url <that-url>`.
  2. Parse the JSON output. Capture `repo`, `workdir`, `base_sha`, `head_sha`.
  3. Plan args = `--repo <workdir> --from <base_sha> --to <head_sha>`. Remove the worktree at the end.
- **Otherwise**, plan args = `--repo "$(pwd)"` plus `$ARGUMENTS` passed through (`--from/--to`, `--commit`, or nothing for the working tree).

### Step 1 — Plan
- Run `ccr-engine plan <plan-args> --run-id $RUN > "$RUNDIR/plan.json"`.
- Read `plan.json`. If `bundles` is empty, tell the user "Nothing to review." and stop (clean up the worktree first if in MR mode).

### Step 2 — Review each bundle (parallel)
- For each object in `plan.bundles`, dispatch a `file-reviewer` subagent with the `Task` tool. Launch them concurrently, at most ~8–10 in flight at once.
- Each subagent's prompt must contain: that single bundle object (verbatim JSON) and an **absolute** `out_path = $RUNDIR/findings/<bundle.id>.json` (expand `$RUNDIR` to the real absolute path in the prompt text).
- If a subagent errors or times out, record the bundle id and continue with the rest.

### Step 3 — Collect (deterministic)
- Run `ccr-engine collect --plan "$RUNDIR/plan.json" --findings-dir "$RUNDIR/findings" --run-id $RUN > "$RUNDIR/positioned.json"`.

### Step 4 — Reflect
- Dispatch the `reflector` subagent (via `Task`) with the contents of `positioned.json` and an absolute `out_path = $RUNDIR/reflected.json`.
- If reflection is skipped or fails, use `positioned.json` in the next step (the engine keeps unscored findings).

### Step 5 — Report
- Run `ccr-engine report --reflected "$RUNDIR/reflected.json" --format md` (fall back to `--reflected "$RUNDIR/positioned.json"` if `reflected.json` is missing).
- Present the rendered markdown to the user verbatim. If any bundle failed in Step 2, append a one-line note about partial coverage.

### Cleanup
- If MR or PR mode: `git -C <repo> worktree remove --force <workdir>`.
- Leave `$RUNDIR` in place for inspection (it is gitignored).
