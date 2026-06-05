# ccr Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `ccr` — a Go review engine plus a Claude Code plugin that reviews a Git changeset (working tree, range, commit, or GitLab MR URL) and prints line-anchored findings with suggested fix diffs.

**Architecture:** A pure, offline, unit-tested Go binary (`ccr-engine`) owns the correctness-critical steps (file selection, rule matching, diff line numbering, finding validation/positioning, rendering). Claude Code subagents owns judgment (find bugs, score confidence). A `/ccr:review` slash command orchestrates the two inside an interactive session. Determinism ends at the engine boundary; orchestration is the slash-command prompt.

**Tech Stack:** Go 1.23, `encoding/json`, `os/exec` (git, glab); deps `github.com/bmatcuk/doublestar/v4` (glob `**` + braces), `github.com/sourcegraph/go-diff/diff` (unified-diff parsing). Plugin = markdown command + subagent files. `glab` for MR mode.

**Spec:** `docs/superpowers/specs/2026-06-05-ccr-design.md`

---

## TDD rhythm (applies to every engine task)

Each engine task follows: **write failing test → run, confirm it fails → write minimal impl → run, confirm pass → commit.** Test/impl code is given per task; the four run/commit steps are implied by this rhythm (not repeated each time). Run all engine tests with `cd engine && go test ./...`. Commit messages use the repo convention (`type: subject` + CHANGES bullets).

## Dependencies & file structure

```
engine/
  go.mod                              module ccr  (go 1.23)
  cmd/ccr-engine/main.go              subcommand dispatch (stdlib flag)
  internal/model/types.go            shared JSON types
  internal/rules/glob.go             glob match (doublestar + brace)
  internal/rules/rules.go            4-layer load + first-match resolve
  internal/rules/system_rules.json   embedded Go-focused defaults (go:embed)
  internal/diffparse/diffparse.go    unified diff -> files, hunks, numbered lines
  internal/gitx/gitx.go              git diff for each mode; worktree helpers
  internal/plan/plan.go              select + bundle + match -> Plan
  internal/collect/collect.go        validate/repair + position + dedup
  internal/report/report.go          render markdown + diff suggestions
  internal/mrprep/mrprep.go          URL parse + glab + worktree (network edge)
  internal/runpaths/runpaths.go      .ccr/tmp/<run-id> path helpers
  testdata/                          fixtures
Makefile
plugin/.claude-plugin/plugin.json
plugin/commands/review.md            -> /ccr:review
plugin/agents/file-reviewer.md
plugin/agents/reflector.md
```

---

## Phase 0 — Foundation

### Task 0.1: Go module + Makefile + main skeleton
**Files:** Create `engine/go.mod`, `engine/cmd/ccr-engine/main.go`, `Makefile`.

- [ ] `engine/go.mod`:
```
module ccr

go 1.23

require (
	github.com/bmatcuk/doublestar/v4 v4.6.1
	github.com/sourcegraph/go-diff v0.7.0
)
```
- [ ] `engine/cmd/ccr-engine/main.go` — dispatch on `os.Args[1]` to subcommands `plan|collect|report|rules|mr-prep|version`; unknown → usage to stderr, exit 2. Each handler is a `func(args []string) error` registered in a `map[string]func([]string) error`. `main` prints error to stderr and exits 1 on error.
- [ ] `Makefile`:
```make
build:
	cd engine && go build -o ../ccr-engine ./cmd/ccr-engine
test:
	cd engine && go test ./...
install: build
	install -m 0755 ccr-engine $(HOME)/.local/bin/ccr-engine
.PHONY: build test install
```
- [ ] Test `engine/cmd/ccr-engine/main_test.go`: `version` subcommand prints a non-empty version and exits 0; unknown subcommand exits non-zero. (Use `exec.Command(go run ...)` or refactor dispatch into a testable `Run(args, stdout, stderr) int`.) Prefer a `Run` function tested directly.
- [ ] Commit: `chore: scaffold ccr-engine go module and dispatch`

### Task 0.2: Shared types
**Files:** Create `engine/internal/model/types.go`, `engine/internal/model/types_test.go`.

- [ ] Define exact JSON contracts from the spec. Struct tags must match spec field names.
```go
package model

type Rule struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // high|medium|low
	Category string `json:"category"`
	Guidance string `json:"guidance"`
}

type ReviewLine struct {
	Line int    `json:"line"`
	Code string `json:"code"`
}

type Bundle struct {
	ID          string                  `json:"id"`
	Files       []string                `json:"files"`
	Rules       []Rule                  `json:"rules"`
	Diff        string                  `json:"diff"`
	ReviewLines map[string][]ReviewLine `json:"review_lines"`
}

type Skipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

type DiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

type MRInfo struct {
	URL      string   `json:"url"`
	IID      int      `json:"iid"`
	Project  string   `json:"project"`
	Host     string   `json:"host"`
	DiffRefs DiffRefs `json:"diff_refs"`
}

type Plan struct {
	RunID   string    `json:"run_id"`
	Mode    string    `json:"mode"` // worktree|range|commit|mr
	Repo    string    `json:"repo"`
	BaseRef string    `json:"base_ref"`
	HeadRef string    `json:"head_ref"`
	MR      *MRInfo   `json:"mr"`
	Bundles []Bundle  `json:"bundles"`
	Skipped []Skipped `json:"skipped"`
}

// Written by the file-reviewer subagent (untrusted input).
type RawFinding struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	RuleID       string `json:"rule_id"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Rationale    string `json:"rationale"`
	SuggestedFix string `json:"suggested_fix"`
}

type BundleFindings struct {
	BundleID string       `json:"bundle_id"`
	Findings []RawFinding `json:"findings"`
}

// After collect.
type PositionedFinding struct {
	ID           string  `json:"id"`
	File         string  `json:"file"`
	Line         int     `json:"line"`
	Anchor       string  `json:"anchor"` // exact|exact-recovered|hunk-fallback
	RuleID       string  `json:"rule_id"`
	Severity     string  `json:"severity"`
	Title        string  `json:"title"`
	Rationale    string  `json:"rationale"`
	SuggestedFix string  `json:"suggested_fix"`
	Confidence   float64 `json:"confidence,omitempty"` // set by reflector
}

type Dropped struct {
	Reason string     `json:"reason"`
	Raw    RawFinding `json:"raw"`
}

type Positioned struct {
	RunID    string              `json:"run_id"`
	Findings []PositionedFinding `json:"findings"`
	Dropped  []Dropped           `json:"dropped"`
}
```
- [ ] Test: round-trip marshal/unmarshal a `Plan` with one bundle + one review line; assert JSON keys equal the spec (`run_id`, `review_lines`, `base_ref`, etc.) via `json.Marshal` + substring asserts. Assert `MR` omitted-as-null when nil (pointer) serializes to `"mr":null`.
- [ ] Commit: `feat: add shared json contract types`

---

## Phase 1 — Rules engine

### Task 1.1: Glob matcher
**Files:** Create `engine/internal/rules/glob.go`, `glob_test.go`.

- [ ] `func Match(pattern, path string) (bool, error)` — wrap `doublestar.Match`. doublestar supports `**` and `{a,b}` natively; normalize `path` to slash with `filepath.ToSlash`.
```go
package rules

import (
	"github.com/bmatcuk/doublestar/v4"
	"path/filepath"
)

func Match(pattern, path string) (bool, error) {
	return doublestar.Match(pattern, filepath.ToSlash(path))
}
```
- [ ] Tests (table-driven): `**/*.go` matches `internal/svc/a.go` (true); `**/*.go` vs `a.ts` (false); `**/*_test.go` vs `internal/x_test.go` (true); brace `**/*.{go,ts}` matches both `a.go` and `b.ts`; `*.go` vs `dir/a.go` (false, single-star no slash).
- [ ] Commit: `feat: add doublestar-based glob matcher`

### Task 1.2: Rule types + single-layer first-match
**Files:** Modify `engine/internal/rules/rules.go`, `rules_test.go`.

- [ ] Layer types + resolver:
```go
type LayerEntry struct {
	Path  string       `json:"path"`
	Rules []model.Rule `json:"rules"`
}
type RuleFile struct {
	Layers []LayerEntry `json:"layers"`
}

// MatchFile returns the rules of the first LayerEntry whose Path matches,
// or (nil,false) if none match in this RuleFile.
func (rf RuleFile) MatchFile(path string) ([]model.Rule, bool)
```
- [ ] Tests: first matching entry wins (declaration order); non-matching falls through within file; empty `rules` on a matching entry returns `([], true)` (file is "covered" with no rules — stops fall-through). Document this semantics in a comment.
- [ ] Commit: `feat: add rule file types and first-match resolution`

### Task 1.3: Four-layer resolution + embedded defaults
**Files:** Modify `engine/internal/rules/rules.go`; create `engine/internal/rules/system_rules.json`.

- [ ] `system_rules.json` — Go-focused defaults + generic catch-all:
```json
{ "layers": [
  { "path": "**/*_test.go", "rules": [
    {"id":"go-test-assert","severity":"low","category":"tests","guidance":"Flag tests with no assertions, or t.Error without t.Fatal where continued execution panics."}
  ]},
  { "path": "**/*.go", "rules": [
    {"id":"go-nil-deref","severity":"high","category":"correctness","guidance":"Flag dereference of a value that may be nil without a prior check; common with funcs returning (T*, error)."},
    {"id":"go-err-unchecked","severity":"medium","category":"correctness","guidance":"Flag ignored errors (assigned to _ or not checked) on calls that can fail meaningfully."},
    {"id":"go-goroutine-leak","severity":"high","category":"concurrency","guidance":"Flag goroutines with no exit path or missing context cancellation; channels that may block forever."},
    {"id":"go-data-race","severity":"high","category":"concurrency","guidance":"Flag shared mutable state accessed without synchronization across goroutines."},
    {"id":"go-ctx-misuse","severity":"medium","category":"correctness","guidance":"Flag context.Background() in request paths, missing ctx propagation, or storing ctx in structs."},
    {"id":"go-sql-injection","severity":"high","category":"security","guidance":"Flag string-concatenated SQL; require parameterized queries."}
  ]},
  { "path": "**/*", "rules": [
    {"id":"generic-correctness","severity":"medium","category":"correctness","guidance":"Flag clear logic errors, resource leaks, missing error handling, and security issues introduced by this change."}
  ]}
]}
```
- [ ] `//go:embed system_rules.json` into `var systemRulesJSON []byte`.
- [ ] `func Resolve(repoDir, ruleFlagPath, homeDir string) (Resolver, error)` loads up to four `RuleFile`s in priority order (flag path, `<repo>/.ccr/rule.json`, `<home>/.ccr/rule.json`, embedded). Missing file → skip; malformed file → error. `Resolver.RulesFor(path) []model.Rule` returns the rules from the highest-priority layer whose entry matches (first-match across layers in priority order).
- [ ] Tests: project layer overrides system for a matching path; missing project/global files skip cleanly; malformed `--rule` file → error; a `.go` path with no project rules falls to embedded `go-nil-deref` set; an unmatched extension hits `generic-correctness` via the `**/*` catch-all.
- [ ] Commit: `feat: add four-layer rule resolver with embedded defaults`

### Task 1.4: `rules check` subcommand
**Files:** Modify `engine/cmd/ccr-engine/main.go`; create `rules_cmd.go` in cmd pkg; test.

- [ ] `ccr-engine rules check <file> [--repo dir] [--rule path]` prints the matched layer source + rule IDs for the path. Output e.g. `internal/svc/a.go -> [go-nil-deref go-err-unchecked ...] (source: embedded)`.
- [ ] Test: `rules check foo.go` in a temp repo with a project `.ccr/rule.json` prints project source; `foo.unknown` prints the catch-all.
- [ ] Commit: `feat: add rules check subcommand`

---

## Phase 2 — Diff parsing & plan

### Task 2.1: Unified diff → numbered review lines
**Files:** Create `engine/internal/diffparse/diffparse.go`, `diffparse_test.go`.

- [ ] Parse a unified diff (string) with `go-diff/diff.ParseMultiFileDiff`. For each file, walk hunks; track the new-file line counter from each hunk's `NewStartLine`. Added (`+`) and context (` `) lines advance the new counter; removed (`-`) lines do not. Emit, per file, the list of **added** lines as `model.ReviewLine{Line, Code}` (review only added/changed lines), and the hunk header new-start lines (for fallback anchoring).
```go
type FileDiff struct {
	NewPath     string
	ReviewLines []model.ReviewLine // added lines, absolute new-file numbers
	HunkStarts  []int              // new-file start line of each hunk (fallback)
}
func Parse(unified string) ([]FileDiff, error)
```
- [ ] Tests: a 2-hunk diff yields correct absolute line numbers for added lines (verify the counter math across context lines); removed-only lines produce no review lines but a HunkStart; renamed/binary files handled (skip binary). Include a fixture diff string in the test.
- [ ] Commit: `feat: parse unified diff into numbered review lines`

### Task 2.2: git invocation per mode
**Files:** Create `engine/internal/gitx/gitx.go`, `gitx_test.go`.

- [ ] Functions shelling to `git -C <repo>`:
```go
func DiffWorktree(repo string) (string, error)        // staged+unstaged vs HEAD: git diff HEAD
func DiffRange(repo, from, to string) (string, error) // git diff <from>...<to>
func DiffCommit(repo, sha string) (string, error)     // git diff <sha>^ <sha>  (or show)
func MergeBase(repo, a, b string) (string, error)     // git merge-base a b
func IsRepo(repo string) bool
```
Use `os/exec`; capture stdout; on non-zero exit return error incl. stderr.
- [ ] Tests: create a temp git repo in test (helper that `git init`, commits a file, edits it). `DiffWorktree` returns a diff containing the edit; `DiffRange`/`DiffCommit` over two commits returns expected paths. Skip if `git` not on PATH (`t.Skip`).
- [ ] Commit: `feat: add git diff helpers for each input mode`

### Task 2.3: File selection (ignore filter)
**Files:** Create `engine/internal/plan/select.go`, `select_test.go`.

- [ ] `func Select(files []string) (kept []string, skipped []model.Skipped)` — drop vendored/generated/lock/binary paths via a default ignore glob set (`**/vendor/**`, `**/node_modules/**`, `**/*.lock`, `**/*.min.*`, `**/dist/**`, `**/*.pb.go`, `**/testdata/**` is KEPT). Reason string `ignored:<pattern>`.
- [ ] Tests: `vendor/x.go` skipped with reason; `internal/a.go` kept; `go.sum` skipped.
- [ ] Commit: `feat: add file selection with default ignore set`

### Task 2.4: Bundling
**Files:** Create `engine/internal/plan/bundle.go`, `bundle_test.go`.

- [ ] v1 bundling: group files that share a directory **and** a base-name stem differing only by a known sibling suffix/locale (e.g. `message_en.properties` + `message_zh.properties`; `x.go` + `x_test.go`). Otherwise one file per bundle. Deterministic order (sort files, sort bundles by first file).
```go
func Bundle(files []string) [][]string
```
- [ ] Tests: `["a/x.go","a/x_test.go"]` → one bundle; `["a/x.go","b/y.go"]` → two bundles; locale pair bundled; output order stable.
- [ ] Commit: `feat: add deterministic file bundling`

### Task 2.5: `plan` subcommand (wire it together)
**Files:** Create `engine/internal/plan/plan.go`, `plan_test.go`; add handler in cmd.

- [ ] `func Build(opts Options) (model.Plan, error)` where Options has Repo, Mode, From, To, Commit, RulePath, RunID, MR. Steps: get diff via gitx by mode → diffparse → Select → Bundle → for each bundle assemble `model.Bundle` (files, per-file ReviewLines, per-bundle sliced diff text, union of `Resolver.RulesFor` across the bundle's files, dedup rules by ID) → set Plan fields. RunID default = timestamp+rand passed in by caller (engine takes `--run-id`; main generates if absent using time — acceptable, this is the impure CLI layer).
- [ ] `ccr-engine plan --repo <dir> [--from --to | --commit | default worktree] [--rule p] [--run-id id]` prints Plan JSON to stdout.
- [ ] Tests: against a temp git repo with two changed `.go` files in different dirs → Plan has 2 bundles, each with `go-nil-deref` rule present and non-empty review_lines; empty diff → Plan with 0 bundles (and main exits 0 but prints a clear note to stderr). Mode `range` over two commits works.
- [ ] Commit: `feat: add plan subcommand assembling review plan`

---

## Phase 3 — Collect (core IP)

### Task 3.1: JSON validate/repair for findings
**Files:** Create `engine/internal/collect/parse.go`, `parse_test.go`.

- [ ] `func LoadFindings(dir string) ([]model.BundleFindings, []model.Dropped)` — read every `*.json` in dir; tolerant decode: if a file fails to parse, attempt to extract the first JSON object via brace-matching; if still bad, record nothing parseable (return a Dropped with reason `schema_invalid` and empty Raw). Validate each finding has non-empty File and Title; coerce missing Severity to `medium`; clamp unknown severity to `medium`.
- [ ] Tests: a clean findings file loads N findings; a file with leading prose + a JSON block recovers the block; a finding missing severity defaults medium; a finding missing file → dropped `schema_invalid`.
- [ ] Commit: `feat: tolerant loader for subagent findings`

### Task 3.2: Constrained-choice positioning
**Files:** Create `engine/internal/collect/position.go`, `position_test.go`.

- [ ] Implement the spec algorithm exactly:
```go
// positionOne resolves a raw finding to a line within its bundle.
// Returns (PositionedFinding, ok). ok=false => dropped (caller records reason).
func positionOne(rf model.RawFinding, b model.Bundle) (model.PositionedFinding, string)
```
Logic:
1. If `rf.File` not in `b.Files`: try snippet recovery across the bundle (see step 3 over all files); if none, return drop reason `file_not_in_bundle`.
2. Let `lines = b.ReviewLines[rf.File]`. If `rf.Line` is in `lines` → anchor `exact`.
3. Else if `rf.SuggestedFix`/cited code present: whitespace-normalized match of the finding's cited code against `lines[i].Code`; best unique match above threshold → that line, anchor `exact-recovered`. (Normalize: strip leading/trailing ws, collapse internal runs.) Use the finding's `Title`+`Rationale` only as last resort; primary signal is any fenced/inline code in the finding — for v1 match against `rf.SuggestedFix` first line and `rf.Rationale` tokens; keep it simple: match `rf` cited code = first non-empty line of `SuggestedFix`.
4. Else → nearest `HunkStart` ≤ rf.Line (or first review line) for that file, anchor `hunk-fallback`.
- [ ] Tests: exact line in set → exact; line not in set but snippet matches a review line → exact-recovered on that line; nothing matches → hunk-fallback (never dropped for line reasons); file not in bundle and no recovery → dropped `file_not_in_bundle`. Whitespace-normalization: `"return  x.Bar()"` matches review line `"return x.Bar()"`.
- [ ] Commit: `feat: constrained-choice line positioning`

### Task 3.3: Dedup + `collect` subcommand
**Files:** Create `engine/internal/collect/collect.go`, `collect_test.go`; add handler.

- [ ] `func Collect(plan model.Plan, findingsDir string) model.Positioned` — map bundle_id→Bundle; for each BundleFindings, position each finding; dedup by `(file,line,rule_id)` keeping highest severity (high>medium>low) and concatenating distinct rationales; assign stable IDs `f%03d` in sorted order (by file, then line). Collect Dropped with reasons.
- [ ] `ccr-engine collect --plan plan.json --findings-dir dir [--run-id id]` prints Positioned JSON.
- [ ] Tests: two findings same file/line/rule → one, highest severity; ordering stable; dropped recorded. End-to-end with a small plan + findings dir fixture.
- [ ] Commit: `feat: add collect subcommand with dedup`

---

## Phase 4 — Report

### Task 4.1: Render markdown + diff suggestions
**Files:** Create `engine/internal/report/report.go`, `report_test.go`; add handler.

- [ ] `func Render(p model.Positioned, minConfidence float64, format string) (string, error)` — filter findings with `Confidence>0 && Confidence<minConfidence` out (Confidence==0 means reflector didn't run → keep); sort by file then severity (high first) then line; group by file. Per finding render:
```
### path/to/file.go:43  [HIGH]  Possible nil dereference  (go-nil-deref)
<rationale>

```diff
<suggested_fix as a fenced diff block, if present>
```
```
Anchor `hunk-fallback` adds `(approx location)`. `format=json` prints the filtered Positioned; `text` strips markdown fences. Header summary line: counts by severity + dropped count.
- [ ] `ccr-engine report --reflected reflected.json [--min-confidence 0.5] [--format md|text|json]` prints to stdout.
- [ ] Tests: high before medium; below-threshold filtered when confidence set; confidence==0 kept; approx label present for hunk-fallback; json format round-trips.
- [ ] Commit: `feat: add report subcommand rendering findings`

---

## Phase 5 — MR mode (network edge)

### Task 5.1: GitLab MR URL parsing
**Files:** Create `engine/internal/mrprep/url.go`, `url_test.go`.

- [ ] `func ParseMRURL(raw string) (host, projectPath string, iid int, err error)` — handle `https://<host>/<group>/<subgroup>/<project>/-/merge_requests/<iid>` (and trailing segments/query). projectPath = everything between host and `/-/merge_requests`.
- [ ] Tests: gitlab.com nested group URL; self-hosted host; trailing `/diffs` or `?` tolerated; non-MR URL → error.
- [ ] Commit: `feat: parse gitlab merge request urls`

### Task 5.2: `mr-prep` via glab + worktree
**Files:** Create `engine/internal/mrprep/mrprep.go`, `mrprep_test.go` (unit-test URL + arg building; integration behind build tag/flag).

- [ ] Steps: `glab api projects/<url-encoded-path>/merge_requests/<iid>` (exec) → decode JSON for `target_branch`, `source_branch`, `sha`, `diff_refs{base_sha,head_sha,start_sha}`. Ensure local repo (compare `git -C . remote get-url origin` host/path; else `glab repo clone <path> <tmp>`). `git -C <repo> fetch origin <head_sha>` then `git -C <repo> worktree add --detach <tmp> <head_sha>`. Print JSON `{workdir, base_sha, head_sha, diff_refs, mr:{...}}`. Provide `func Cleanup(repo, worktree string)` doing `git worktree remove --force`.
- [ ] Detect missing `glab` (exec.LookPath) and unauthenticated (`glab auth status` non-zero) → actionable errors.
- [ ] Tests (unit): the glab arg vector for a parsed URL is correct; JSON decode of a sample MR payload yields diff_refs. Integration test gated by `CCR_GLAB_IT=1` env (skipped by default).
- [ ] Commit: `feat: add mr-prep glab + worktree materialization`

---

## Phase 6 — Plugin

### Task 6.1: plugin.json + subagents
**Files:** Create `plugin/.claude-plugin/plugin.json`, `plugin/agents/file-reviewer.md`, `plugin/agents/reflector.md`.

- [ ] `plugin.json`: name `ccr`, description, version `0.1.0`, `commands: "./commands"`, `agents: "./agents"`.
- [ ] `file-reviewer.md` frontmatter: `name: file-reviewer`, `description`, `tools: Read, Grep, Glob` (read-only). Body: given one bundle (files, rules+guidance, diff, review_lines menu) — find issues matching rules and clear bugs; **cite each finding's `line` by choosing from review_lines** (never invent a number); output a JSON object matching `BundleFindings` and write it to the path the orchestrator gives; one finding per real issue; prefer precision over recall; include a `suggested_fix` snippet whose first line is the exact cited code when possible (helps positioning recovery).
- [ ] `reflector.md` frontmatter: `name: reflector`, `tools: Read`. Body: given positioned findings, for each judge if real given the diff (read the file if needed); assign `confidence` 0..1; default low when unsure; output the findings array with `confidence` added.
- [ ] No test (markdown). Commit: `feat: add ccr plugin manifest and subagents`

### Task 6.2: `/ccr:review` command
**Files:** Create `plugin/commands/review.md`.

- [ ] Frontmatter: `description`, `argument-hint: "[--from X --to Y | --commit SHA | <gitlab-mr-url>]"`, `allowed-tools: Bash, Read, Grep, Glob, Task`.
- [ ] Body (orchestration prose the model executes), explicit steps:
  1. Parse `$ARGUMENTS`. If it's a GitLab MR URL → run `ccr-engine mr-prep --url <url>`, capture `workdir`/`base_sha`/`head_sha`; set plan args `--repo <workdir> --from <base_sha> --to <head_sha>`. Else map flags to plan args (default worktree). Generate a run id.
  2. Run `ccr-engine plan <args> --run-id <id>` → save JSON to `.ccr/tmp/<id>/plan.json`. If 0 bundles → tell user "nothing to review" and stop.
  3. For each bundle, dispatch a `file-reviewer` subagent **in parallel (≤10 at a time)** passing that bundle's slice and the output path `.ccr/tmp/<id>/findings/<bundle>.json`. If a bundle errors/times out, note it and continue.
  4. Run `ccr-engine collect --plan .../plan.json --findings-dir .../findings --run-id <id>` → `.ccr/tmp/<id>/positioned.json`.
  5. Dispatch the `reflector` subagent over positioned.json → write `.ccr/tmp/<id>/reflected.json`.
  6. Run `ccr-engine report --reflected .../reflected.json --format md` and present the output to the user. Mention partial coverage if any bundle failed. If MR mode, `git worktree remove --force <workdir>` to clean up.
- [ ] Commit: `feat: add /ccr:review orchestration command`

---

## Phase 7 — End-to-end & docs

### Task 7.1: Fixture golden flow (engine only)
**Files:** Create `engine/testdata/fixture_repo/` setup helper + `engine/e2e_test.go`.

- [ ] Test builds a temp git repo with a planted nil-deref in a `.go` file, runs `plan` (in-proc), hand-writes a findings JSON citing the correct review line, runs `collect`, then `report`, and asserts the report contains the file:line and severity. This exercises the deterministic chain without a model.
- [ ] Commit: `test: end-to-end deterministic chain over fixture`

### Task 7.2: README + install notes
**Files:** Create `README.md`.

- [ ] Document: build (`make build`, put `ccr-engine` on PATH), install plugin (`/plugin` or copy to `.claude/`), usage of `/ccr:review` for each mode, `glab` prereq for MR mode, rule customization (`.ccr/rule.json`). No mention of any third-party prior art.
- [ ] Commit: `docs: add README with build, install, usage`

---

## Self-review (run after writing; fix inline)

- **Spec coverage:** plan(§5.1)→2.5; file-reviewer output(§5.2)→6.1; collect(§5.3)→3.x; reflector(§5.4)→6.1+4.1 filter; report(§5.5)→4.1; rules check(§5.6)→1.4; mr-prep(§5.7)→5.x; subagents(§6)→6.1; rules(§7)→1.x; errors(§8)→handled across plan/collect/report/mr-prep; testing(§9)→tests each task +7.1; layout(§11)→matches; prereqs(§12)→7.2. No gaps.
- **Placeholders:** none — every task has files, test intent, and code/signatures.
- **Type consistency:** field names (`review_lines`, `base_ref`, `diff_refs`, `rule_id`, `suggested_fix`, anchors `exact|exact-recovered|hunk-fallback`) consistent across model, collect, report, plugin.
