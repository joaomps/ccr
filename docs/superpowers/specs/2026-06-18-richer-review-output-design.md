# Richer review output — detection heuristics + confidence tiers + narrative

## Context

Porting the valued parts of the `gitlab-mr-review` plugin into ccr. The user
reported "good output" from that plugin and asked to bring its features over.
Delta analysis: ccr already has multi-agent review (file-reviewer + reflector),
confidence scoring, dedup, and changed-lines-only scope. The real gaps are:

1. ccr's `file-reviewer` is **thin** (apply rules, cite a line, precision-first)
   vs that plugin's rich `bug-detection` heuristics (structural blast-radius +
   holistic passes) — which is what actually surfaced the good findings.
2. ccr emits a **flat** confidence-filtered list; the plugin layers output into
   tiers + narrative sections (summary, impact, focus, "what wasn't checked").

The user explicitly opted into both, including the precision→recall philosophy
shift (eyes open: it changes ccr's documented "precision-first" character).

## Decisions

1. **`plugin/agents/file-reviewer.md`** (prompt only): add a distilled
   structural blast-radius pass (caller/consumer consistency, parallel-path
   divergence, writer-value → misleading-output trace, try/catch scope leakage,
   SQL-join cardinality), a holistic re-read pass (regression-of-prior-fix,
   diverged invariants, diagnostic-message quality, doc/comment contradiction),
   and the cluster rule (re-read ±10 lines after a hit). Flip precision-first →
   **over-flag-then-gate**: report everything plausible; the `reflector` +
   `--min-confidence` own the gate. **Keep** the hard invariants: cite only from
   `review_lines`, changed lines only (so no `origin` field is needed — ccr's
   scope already excludes pre-existing code). Stylistic-without-CLAUDE.md → nit.

2. **`engine/internal/report/report.go`** (`renderMarkdown` only): tier the kept
   findings — **Suggested** (confidence ≥ 0.8), **Review before acting**
   (min ≤ confidence < 0.8), `< min` dropped (unchanged). `confidence == 0`
   (reflector skipped) → single flat list, as today. `--format json` and `text`
   are **untouched** — the eval scorer (`recall.sh`/`consistency.sh`) keys on
   `.findings[]`; re-tiering JSON would break it. Update `report_test.go`.

3. **`plugin/commands/review.md`** (prompt only): after the engine report, the
   orchestrator synthesizes narrative sections from the diff + `reflected.json` +
   plan metadata it already holds — high-level summary, impact / areas to watch,
   review focus areas, and **what this review didn't check** (untracked files,
   files matched to no rules, errored bundles, skipped passes).

## Out of scope

Posting findings back to the MR/PR, new agents, GitLab-history dimensions
(historical-context / mr-history), `origin`/`category` model fields.

## Risks & validation

- **Over-flag risk**: more false positives could survive. Mitigation: the
  `reflector` is the gate; the tiers separate high-confidence from
  review-before-acting so users still get a clean default lane.
- This is a recall/precision experiment — **validate with the eval harness**:
  `make eval` (recall on fixtures must stay 3/3; consistency not tanked) plus a
  before/after review on a real ccr commit.
- Tier high-threshold `0.8` is hardcoded; `--min-confidence` (default 0.5) is the
  lower bound. Revisit thresholds if eval shows drift.

## Verification

`make test` (report change + updated `report_test.go`), `make eval`, and a
before/after `/ccr:review` on a real ccr commit to confirm the heuristics lift
findings without wrecking consistency.
