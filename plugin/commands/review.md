---
description: Hybrid code review of the current changes (or a GitLab MR URL) — line-anchored findings with suggested fix diffs. Does not modify source files.
argument-hint: "[--from REF --to REF | --commit SHA | <gitlab-mr-url>]"
allowed-tools: Bash, Read, Grep, Glob, Task
---

Run a code review using the `ccr-engine` binary for the deterministic steps and the `file-reviewer` / `reflector` subagents for judgment. Determinism lives in the engine; you are the orchestrator gluing the steps together.

**Prerequisite:** `ccr-engine` must be on PATH (`make install` from the ccr repo). For MR mode, `glab` must be installed and authenticated.

Arguments: `$ARGUMENTS`

Pick a short run id `RUN` (e.g. a timestamp like `20260605-104500`). All artifacts go under `.ccr/tmp/$RUN/` in the current repo (add `.ccr/` to the repo's `.gitignore`). These are temporary files, not source edits.

### Step 0 — Resolve input mode
- **If `$ARGUMENTS` contains a GitLab MR URL** (matches `/-/merge_requests/`):
  1. Run `ccr-engine mr-prep --url <that-url>`.
  2. Parse the JSON output. Capture `repo`, `workdir`, `base_sha`, `head_sha`.
  3. Plan args = `--repo <workdir> --from <base_sha> --to <head_sha>`. Remember to remove the worktree at the end.
- **Otherwise**, plan args = `--repo .` plus `$ARGUMENTS` passed through (`--from/--to`, `--commit`, or nothing for the working tree).

### Step 1 — Plan
- `mkdir -p .ccr/tmp/$RUN/findings`
- Run `ccr-engine plan <plan-args> --run-id $RUN > .ccr/tmp/$RUN/plan.json`.
- Read `plan.json`. If `bundles` is empty, tell the user "Nothing to review." and stop (clean up the worktree first if in MR mode).

### Step 2 — Review each bundle (parallel)
- For each object in `plan.bundles`, dispatch a `file-reviewer` subagent. Launch them concurrently, at most ~8–10 in flight at once.
- Each subagent's prompt must contain: that single bundle object (verbatim JSON) and `out_path = .ccr/tmp/$RUN/findings/<bundle.id>.json`.
- If a subagent errors or times out, record the bundle id and continue with the rest.

### Step 3 — Collect (deterministic)
- Run `ccr-engine collect --plan .ccr/tmp/$RUN/plan.json --findings-dir .ccr/tmp/$RUN/findings --run-id $RUN > .ccr/tmp/$RUN/positioned.json`.

### Step 4 — Reflect
- Dispatch the `reflector` subagent with the contents of `positioned.json` and `out_path = .ccr/tmp/$RUN/reflected.json`.
- If reflection is skipped or fails, use `positioned.json` in the next step (the engine keeps unscored findings).

### Step 5 — Report
- Run `ccr-engine report --reflected .ccr/tmp/$RUN/reflected.json --format md` (fall back to `--reflected .ccr/tmp/$RUN/positioned.json` if `reflected.json` is missing).
- Present the rendered markdown to the user verbatim. If any bundle failed in Step 2, append a one-line note about partial coverage.

### Cleanup
- If MR mode: `git -C <repo> worktree remove --force <workdir>`.
- Leave `.ccr/tmp/$RUN/` in place for inspection (it is gitignored).
