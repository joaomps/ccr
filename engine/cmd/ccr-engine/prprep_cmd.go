package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"ccr/internal/prprep"
)

// cmdPRPrep handles: ccr-engine pr-prep --url <github-pr-url> [--repo dir]
// It prints a JSON object the orchestrator feeds into `plan`.
func cmdPRPrep(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pr-prep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	prURL := fs.String("url", "", "github pull request url")
	repo := fs.String("repo", ".", "candidate local repo (cloned if it is not the PR project)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *prURL == "" {
		fmt.Fprintln(stderr, "usage: ccr-engine pr-prep --url <github-pr-url> [--repo dir]")
		return 2
	}

	res, err := prprep.Prep(*prURL, *repo)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(res); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
