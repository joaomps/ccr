// Package plan assembles a deterministic review plan from a git changeset:
// it selects files, groups them into bundles, numbers the reviewable lines,
// and matches rules to each bundle.
package plan

import (
	"fmt"
	"sort"
	"strings"

	"ccr/internal/diffparse"
	"ccr/internal/gitx"
	"ccr/internal/model"
	"ccr/internal/rules"
)

// Options configures a plan build.
type Options struct {
	Repo     string
	Mode     string // worktree|range|commit|mr
	From     string
	To       string
	Commit   string
	RulePath string
	HomeDir  string
	RunID    string
	MR       *model.MRInfo
}

// Build produces the review plan. It is pure with respect to the model layer:
// the only side effects are read-only git invocations against opts.Repo.
func Build(opts Options) (model.Plan, error) {
	if !gitx.IsRepo(opts.Repo) {
		return model.Plan{}, fmt.Errorf("not a git repository: %s", opts.Repo)
	}

	var (
		raw  string
		err  error
		mode = opts.Mode
		base = opts.From
		head = opts.To
	)
	switch mode {
	case "range", "mr":
		raw, err = gitx.DiffRange(opts.Repo, opts.From, opts.To)
	case "commit":
		raw, err = gitx.DiffCommit(opts.Repo, opts.Commit)
		base, head = opts.Commit+"^", opts.Commit
	default:
		mode = "worktree"
		raw, err = gitx.DiffWorktree(opts.Repo)
		base, head = "HEAD", "WORKTREE"
	}
	if err != nil {
		return model.Plan{}, err
	}

	fileDiffs, err := diffparse.Parse(raw)
	if err != nil {
		return model.Plan{}, err
	}

	var paths []string
	diffByPath := make(map[string]diffparse.FileDiff, len(fileDiffs))
	for _, fd := range fileDiffs {
		paths = append(paths, fd.NewPath)
		diffByPath[fd.NewPath] = fd
	}

	kept, skipped := Select(paths)
	bundles := Bundle(kept)

	res, err := rules.Resolve(opts.Repo, opts.RulePath, opts.HomeDir)
	if err != nil {
		return model.Plan{}, err
	}

	p := model.Plan{
		RunID:   opts.RunID,
		Mode:    mode,
		Repo:    opts.Repo,
		BaseRef: base,
		HeadRef: head,
		MR:      opts.MR,
		Skipped: skipped,
	}

	for i, files := range bundles {
		b := model.Bundle{
			ID:          fmt.Sprintf("b%03d", i+1),
			Files:       files,
			ReviewLines: map[string][]model.ReviewLine{},
		}
		ruleSet := map[string]model.Rule{}
		var diffParts []string
		for _, f := range files {
			fd := diffByPath[f]
			b.ReviewLines[f] = fd.ReviewLines
			diffParts = append(diffParts, fd.Raw)
			for _, r := range res.RulesFor(f) {
				ruleSet[r.ID] = r
			}
		}
		b.Diff = strings.Join(diffParts, "\n")

		ids := make([]string, 0, len(ruleSet))
		for id := range ruleSet {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			b.Rules = append(b.Rules, ruleSet[id])
		}

		p.Bundles = append(p.Bundles, b)
	}

	return p, nil
}
