# ccr — Design Spec

- **Date:** 2026-06-05
- **Status:** Approved (design); pending implementation plan
- **Component:** `ccr` — a hybrid code-review tool delivered as a Claude Code plugin

## 1. Summary

`ccr` reviews a Git changeset and produces line-anchored review comments with
suggested fixes. It combines a **deterministic engine** (a Go binary that handles
the correctness-critical, non-judgment steps) with **Claude Code subagents** (which
handle judgment: is this a bug, how would you fix it).

It runs **interactively inside a Claude Code CLI session** — the user's logged-in
subscription is the inference engine. No separate API key, no headless invocation,
no token handling.

Four input modes:

- working-tree changes (default)
- branch range (`--from <ref> --to <ref>`)
- single commit (`--commit <sha>`)
- a **GitLab Merge Request URL** (resolved via `glab`)

v1 output is a **terminal report** with suggested fix diffs. It never modifies
source files.

## 2. Design principle: where determinism ends

Determinism lives **inside the engine binary**. The engine computes the things a
language model must not get wrong: which files to review, which rules apply, and
the exact line a comment attaches to.

The **orchestration** — fan out subagents, collect their findings, feed them back
to the engine, render — is the `/ccr:review` slash-command prompt, which the model
interprets. The engine cannot spawn subagents; only a live Claude Code session can.
So the architecture is **deterministic islands in a model-mediated river**.

This is acceptable because `ccr` is interactive: the user sees and can correct
mis-orchestration. The guarantee `ccr` makes is narrow and honest: *the
correctness-critical computations are hard-coded and testable; the glue is
model-driven.*

A direct consequence, applied throughout: **the model never invents a line number,
and the engine never trusts the model's output shape.** Findings are validated and
repaired by code before they reach the report.

## 3. Architecture

Three parts:

1. **`ccr-engine`** — a Go CLI binary. Pure, offline, unit-testable (except the
   one isolated `mr-prep` subcommand, which is the only part that touches the
   network). Subcommands: `plan`, `collect`, `report`, `rules`, `mr-prep`.
2. **`/ccr:review`** — a Claude Code slash command. Orchestrates the pipeline
   inside the user's session. Namespaced (`ccr:`) to avoid colliding with the
   built-in `/code-review`.
3. **Subagents** — `file-reviewer` (reviews one bundle, read-only tools) and
   `reflector` (adversarial confidence check on findings).

Language choice: **Go** for the engine — single static binary, no runtime deps,
strong stdlib for diff parsing and path globbing. Swappable; the plugin contract
is only "a binary that reads Git refs / JSON and emits JSON".

## 4. Data flow

```
/ccr:review [args | gitlab-mr-url]      (live session = orchestrator, model-mediated)
   │
   ▼ [0] ccr-engine mr-prep --url <url>          ← MR mode only; glab + git; ISOLATED edge
   │      resolves URL → SHAs, materializes a detached git worktree at MR head
   │      → {workdir, base_sha, head_sha, diff_refs}
   │
   ▼ [1] ccr-engine plan ...                     ← DETERMINISTIC
   │      → review-plan.json
   │        • file selection (+ ignore filtering)
   │        • bundling (related files grouped)
   │        • rule matching per bundle (4-layer, first-match-wins)
   │        • per-bundle unified diff
   │        • review_lines: every reviewable line numbered (the "menu")
   │
   ▼ [2] fan out file-reviewer subagent per bundle   ← MODEL (≤10 parallel)
   │      read-only tools, isolated context.
   │      Each finding cites a line CHOSEN FROM review_lines.
   │      → writes .ccr/tmp/<run>/findings/<bundle>.json
   │
   ▼ [3] ccr-engine collect                       ← DETERMINISTIC
   │      reads plan + all findings files:
   │        • validate / repair JSON shape
   │        • validate cited line ∈ bundle review_lines
   │          (whitespace-normalized snippet match = recovery; else hunk fallback)
   │        • dedup
   │      → positioned.json
   │
   ▼ [4] reflector subagent                       ← MODEL
   │      per finding: real? confidence 0..1
   │      → .ccr/tmp/<run>/reflected.json
   │
   ▼ [5] ccr-engine report                        ← DETERMINISTIC
   │      filter by confidence threshold, render:
   │      findings by file:line + severity + suggested fix (unified-diff block)
   │      → terminal (markdown). NO source writes.
```

Steps 1, 3, 5 are deterministic engine calls. Steps 2, 4 are model. Step 0 is the
isolated network edge. The arrows between them are the slash-command prompt.

## 5. Engine subcommands and JSON contracts

### 5.1 `ccr-engine plan`

```
ccr-engine plan --repo <dir> [--from <ref> --to <ref> | --commit <sha> | (default: worktree)]
               [--rule <path>] [--run-id <id>]
```

Emits `review-plan.json`:

```json
{
  "run_id": "2026-06-05T10-45-00-ab12",
  "mode": "worktree|range|commit|mr",
  "repo": "/abs/workdir",
  "base_ref": "<sha-or-ref>",
  "head_ref": "<sha-or-ref>",
  "mr": null,
  "bundles": [
    {
      "id": "b001",
      "files": ["internal/svc/a.go", "internal/svc/b.go"],
      "rules": [
        {"id": "go-nil-deref", "severity": "high", "category": "correctness",
         "guidance": "Flag dereference of a value that can be nil without a prior check."}
      ],
      "diff": "<unified diff for this bundle>",
      "review_lines": {
        "internal/svc/a.go": [
          {"line": 42, "code": "x := foo()"},
          {"line": 43, "code": "return x.Bar()"}
        ]
      }
    }
  ],
  "skipped": [{"file": "vendor/x.go", "reason": "ignored:glob"}]
}
```

`review_lines` lists only the added/changed (reviewable) lines, each with its
absolute new-file line number. This is the menu the `file-reviewer` chooses from.

In `mr` mode the top-level `mr` field holds `{url, iid, project, host, diff_refs}`
as produced by `mr-prep`; in all other modes it is `null`.

### 5.2 `file-reviewer` output (written by subagent, read by engine)

`.ccr/tmp/<run>/findings/<bundle>.json`:

```json
{
  "bundle_id": "b001",
  "findings": [
    {
      "file": "internal/svc/a.go",
      "line": 43,
      "rule_id": "go-nil-deref",
      "severity": "high",
      "title": "Possible nil dereference",
      "rationale": "foo() may return nil alongside a non-nil error; x.Bar() is dereferenced unchecked.",
      "suggested_fix": "if x == nil {\n\treturn err\n}\n"
    }
  ]
}
```

`suggested_fix` is optional. The engine treats this whole file as untrusted input.

### 5.3 `ccr-engine collect`

```
ccr-engine collect --plan <plan.json> --findings-dir <dir> --run-id <id>
```

Algorithm per raw finding:

1. **Shape:** validate/repair against the finding schema. Unrecoverable → `dropped` with `reason: schema_invalid`.
2. **File:** `file` must be in the bundle's `files`. If not, attempt recovery by snippet search across the bundle; else drop.
3. **Line (constrained choice):**
   - `line` ∈ `review_lines[file]` → `anchor: "exact"`.
   - else, if a code snippet is present: whitespace-normalized fuzzy match against `review_lines[file]`; best match above threshold → that line, `anchor: "exact-recovered"`.
   - else → nearest hunk header line for `file`, `anchor: "hunk-fallback"` (flagged approximate; never silently dropped).
4. **Dedup:** same `(file, line, rule_id)` collapses; keep highest severity, merge rationales.

Emits `positioned.json`:

```json
{
  "run_id": "...",
  "findings": [
    {"id": "f001", "file": "internal/svc/a.go", "line": 43, "anchor": "exact",
     "rule_id": "go-nil-deref", "severity": "high", "title": "...",
     "rationale": "...", "suggested_fix": "..."}
  ],
  "dropped": [{"reason": "line_not_in_set", "raw": {}}]
}
```

### 5.4 `reflector` output

`.ccr/tmp/<run>/reflected.json` — `positioned.findings` each annotated with a
single `confidence` (0..1) signal. Filtering happens later in `report` via
`--min-confidence`, so confidence is the one source of truth (no separate keep
flag). The reflector is adversarial: default to low confidence when unsure.

### 5.5 `ccr-engine report`

```
ccr-engine report --reflected <reflected.json> [--min-confidence 0.5] [--format text|md|json]
```

Filters by confidence, groups by file then severity, renders each finding as
`file:line  [severity]  title` + rationale + suggested fix as a fenced unified-diff
block. Writes to stdout. **No source-file modification.**

### 5.6 `ccr-engine rules check <file>`

Debugging aid: prints which layer/rule matches a path, for verifying glob
resolution. Doubles as a test surface.

### 5.7 `ccr-engine mr-prep` (isolated network edge)

```
ccr-engine mr-prep --url <gitlab-mr-url>
```

1. Parse URL → host, project path, MR iid.
2. `glab api projects/<url-encoded-path>/merge_requests/<iid>` → `target_branch`,
   `source_branch`, `diff_refs {base_sha, head_sha, start_sha}`.
3. Ensure repo present locally (origin matches host/project); else
   `glab repo clone` into a temp dir.
4. `git fetch origin <head_sha>` then `git worktree add <tmp> <head_sha>` — a
   **detached worktree**, so the user's current branch and working tree are never
   touched. Removed after the run.
5. Print `{workdir, base_sha, head_sha, diff_refs, mr:{...}}`.

The caller then runs `plan --repo <workdir> --from <base_sha> --to <head_sha>`. The
rest of the pipeline is identical across all four input modes.

`diff_refs` is captured even though v1 only prints to the terminal — it is exactly
what GitLab's inline-discussion API needs, so posting-back can be added later
without re-plumbing.

## 6. Subagents

### `file-reviewer`
- **Input:** one bundle (files, matched rules + guidance, unified diff, `review_lines`).
- **Tools:** Read, Grep, Glob — read-only. No Edit, Write, or mutating Bash.
- **Job:** find issues matching the bundle's rules (and clear correctness bugs);
  cite each at a line drawn from `review_lines`; give a rationale and an optional
  fix snippet; write the findings JSON file.
- **Isolation:** its own context window; only the findings file returns to the
  orchestrator.

### `reflector`
- **Input:** `positioned.json`.
- **Tools:** Read.
- **Job:** for each finding, judge whether it is real given the diff (and a file
  read if needed); emit a confidence score; default low when uncertain.

## 7. Review rules

Four-layer resolution, **first-match-wins** within a layer, highest priority first:

| Priority | Source | Path |
|---|---|---|
| 1 | `--rule` flag | user-specified |
| 2 | project | `<repo>/.ccr/rule.json` |
| 3 | global | `~/.ccr/rule.json` |
| 4 | built-in | embedded `system_rules.json` |

Format:

```json
{
  "layers": [
    {
      "path": "**/*.go",
      "rules": [
        {"id": "go-nil-deref", "severity": "high", "category": "correctness",
         "guidance": "Flag dereference of a value that can be nil without a prior check."},
        {"id": "go-goroutine-leak", "severity": "high", "category": "concurrency",
         "guidance": "Goroutines with no exit path / missing context cancellation."}
      ]
    },
    { "path": "**/*_test.go", "rules": [] }
  ]
}
```

- `path` supports `**` recursion and `{go,ts}` brace expansion.
- Rules within a layer are evaluated in declaration order; first match wins.
- A missing rule file is skipped (falls through to the next layer); a malformed
  one is an explicit validation error.
- Built-in `system_rules.json` ships a **Go-focused** default set (nil-deref,
  data race / goroutine leak, unwrapped errors, context misuse, SQL injection,
  plus a generic catch-all for other extensions).

## 8. Error handling

- No repo / empty diff → engine exits non-zero with a clear message; the command
  reports "nothing to review".
- A bundle's subagent fails or times out → that bundle is marked failed; other
  bundles continue; the report notes partial coverage.
- Positioning cannot place a finding → hunk-level fallback, flagged approximate;
  never silently dropped.
- Malformed rule file → explicit error, fall through to next layer.
- Large changeset → bundles processed in waves of ≤10 (subagent concurrency cap).
- `mr-prep`: missing `glab` / not authenticated / MR not found / private →
  fail with the specific reason. Self-hosted host is parsed from the URL.

## 9. Testing

- **Engine (Go) — where most tests live, since it is the IP.** Table-driven units:
  glob + layer-precedence resolution; diff parsing and line numbering; constrained-
  choice positioning (exact, recovered, duplicate-snippet, not-found fallback);
  bundling; JSON validate/repair. Fully deterministic, no model in the loop.
- **Flow:** a golden test over a fixture repo seeded with known issues, asserting
  findings land on the expected lines.
- **`mr-prep`:** integration-tested behind a flag (requires `glab` auth); not part
  of the offline unit suite.

## 10. Scope

**v1 in:** file selection, bundling, 4-layer rules, constrained-choice
positioning, reflection pass, terminal report + suggested diffs, Go default
ruleset, parallel subagents, GitLab MR mode (terminal output).

**Out (later):** posting comments back to the MR, auto-applying fixes, CI /
headless execution, a web session viewer, non-Go rulesets, telemetry.

## 11. Project layout

```
claude-code-review/                      (git repo, default branch main)
├── engine/                              Go module → ccr-engine
│   ├── cmd/ccr-engine/                  main + subcommand wiring
│   ├── internal/plan/                   select, bundle, rulematch, diff numbering
│   ├── internal/collect/               validate/repair, position, dedup
│   ├── internal/report/                markdown + unified-diff rendering
│   ├── internal/rules/                 4-layer resolver + glob
│   ├── internal/mrprep/                glab + git worktree (network edge)
│   └── system_rules.json               embedded defaults
├── plugin/
│   ├── .claude-plugin/plugin.json
│   ├── commands/review.md              → /ccr:review
│   └── agents/
│       ├── file-reviewer.md
│       └── reflector.md
├── docs/superpowers/specs/             this spec
├── Makefile                            build → ccr-engine on PATH
└── .gitignore                          ccr-engine binary, .ccr/, .DS_Store
```

Runtime temp artifacts live under `<repo>/.ccr/tmp/<run-id>/` (gitignored). These
are the only writes in v1 and are not source files.

## 12. Prerequisites

- Claude Code CLI, logged in (subscription).
- Go toolchain (to build `ccr-engine`).
- `glab` installed and authenticated — **only** for MR-URL mode (self-hosted:
  `glab auth login --hostname <host>`).

## 13. Open questions / future

- Comment posting back to a GitLab MR (`--post`) using captured `diff_refs`.
- Smart-bundling heuristics beyond directory/name pairing.
- Auto-apply-fix mode (would change the "no source writes" contract; opt-in).
- CI / headless mode (requires resolving the subscription OAuth-refresh limitation
  or switching to an API key for that path).
