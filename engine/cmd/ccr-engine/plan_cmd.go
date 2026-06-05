package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"ccr/internal/plan"
)

// cmdPlan handles: ccr-engine plan --repo dir [--from X --to Y | --commit SHA] [--rule p] [--run-id id]
func cmdPlan(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "git repository root")
	from := fs.String("from", "", "source ref")
	to := fs.String("to", "", "target ref")
	commit := fs.String("commit", "", "single commit sha")
	rule := fs.String("rule", "", "custom rule json path")
	runID := fs.String("run-id", "", "run id (default: timestamp)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	mode := "worktree"
	switch {
	case *commit != "":
		mode = "commit"
	case *from != "" && *to != "":
		mode = "range"
	}

	id := *runID
	if id == "" {
		id = time.Now().UTC().Format("20060102-150405")
	}
	home, _ := os.UserHomeDir()

	p, err := plan.Build(plan.Options{
		Repo:     *repo,
		Mode:     mode,
		From:     *from,
		To:       *to,
		Commit:   *commit,
		RulePath: *rule,
		HomeDir:  home,
		RunID:    id,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(p.Bundles) == 0 {
		fmt.Fprintln(stderr, "ccr: no reviewable changes found")
	}
	return 0
}
