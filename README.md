# ccr

Hybrid code review, run from inside Claude Code on your own subscription.

A small deterministic Go engine (`ccr-engine`) owns the steps a language model
must not get wrong — which files to review, which rules apply, the exact line a
comment attaches to. Claude Code subagents own the judgment — finding real
issues and scoring confidence. A `/ccr:review` slash command orchestrates the
two inside an interactive session, so reviews use your logged-in Claude Code
subscription with no API key and no headless token handling.

Output is a terminal report with line-anchored findings and suggested fix diffs.
It never modifies your source files.

## Pipeline

```
/ccr:review            (your live Claude Code session = orchestrator)
  └─ ccr-engine plan        select files · bundle · match rules · number diff lines
  └─ file-reviewer ×N       per-bundle review, parallel, cites lines from a fixed menu
  └─ ccr-engine collect     validate/repair findings · anchor to real lines · dedup
  └─ reflector              adversarial confidence score per finding
  └─ ccr-engine report      markdown findings + suggested fix diffs (no source writes)
```

Deterministic steps are the engine; review and reflection are the model. The
glue is the slash-command prompt.

## Install

Build the engine and put it on your PATH:

```sh
make install            # builds ccr-engine into ~/.local/bin
# ensure ~/.local/bin is on PATH, or: make build && cp ccr-engine /usr/local/bin
```

Install the plugin into Claude Code (project-local shown; or copy to `~/.claude`):

```sh
mkdir -p .claude
cp -r plugin/commands plugin/agents .claude/
```

Add `.ccr/` to the target repo's `.gitignore` (run artifacts live in `.ccr/tmp/`).

For GitLab MR review, install and authenticate `glab`:

```sh
glab auth login                       # gitlab.com
glab auth login --hostname <host>     # self-hosted
```

## Usage

In a Claude Code session inside the repo you want to review:

```
/ccr:review                                   # working-tree changes
/ccr:review --from main --to my-feature       # branch range
/ccr:review --commit a1b2c3d                  # a single commit
/ccr:review https://gitlab.com/grp/proj/-/merge_requests/42   # a GitLab MR
```

MR mode resolves the MR with `glab`, checks the MR head out into a detached git
worktree (your current branch is never touched), reviews it, and removes the
worktree afterward.

### Limitations (v1)

- Working-tree mode reviews tracked changes (`git diff HEAD`); brand-new
  **untracked** files are not reviewed until staged or committed.
- Findings are printed to the terminal; posting comments back to a GitLab MR is
  not yet implemented (the MR diff refs are captured for when it is).
- Default rules are Go-focused plus a generic catch-all; add `.ccr/rule.json`
  for other languages.

## Rules

Rules are matched to files by glob and resolved through a four-layer,
first-match-wins chain (highest priority first):

| Priority | Source                         |
|----------|--------------------------------|
| 1        | `--rule <path>`                |
| 2        | `<repo>/.ccr/rule.json`        |
| 3        | `~/.ccr/rule.json`             |
| 4        | built-in defaults (Go-focused) |

Inspect what matches a file:

```sh
ccr-engine rules check internal/svc/handler.go
```

Rule file format:

```json
{
  "layers": [
    { "path": "**/*.go", "rules": [
      { "id": "go-nil-deref", "severity": "high", "category": "correctness",
        "guidance": "Flag dereference of a value that may be nil without a prior check." }
    ]},
    { "path": "**/*_test.go", "rules": [] }
  ]
}
```

`path` supports `**` and `{go,ts}` brace expansion. The first matching layer
entry wins; an empty `rules` list marks files covered with no rules.

## Engine subcommands

`ccr-engine` is usable directly for scripting or debugging:

| Command | Purpose |
|---------|---------|
| `plan`     | Build the review plan JSON from a changeset |
| `collect`  | Validate + line-anchor + dedup subagent findings |
| `report`   | Render findings as `md`, `text`, or `json` |
| `rules check <file>` | Show which rule layer matches a path |
| `mr-prep --url <url>` | Resolve a GitLab MR into a local worktree |

## Develop

```sh
make test     # go test ./...
make vet
make build
```

Engine layout: `internal/plan` (select, bundle, number, match), `internal/collect`
(validate, position, dedup), `internal/report`, `internal/rules`, `internal/diffparse`,
`internal/gitx`, `internal/mrprep`. Design and plan docs live under
`docs/superpowers/`.
