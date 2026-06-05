package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"ccr/internal/mrprep"
)

// cmdMRPrep handles: ccr-engine mr-prep --url <gitlab-mr-url> [--repo dir]
// It prints a JSON object the orchestrator feeds into `plan`.
func cmdMRPrep(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mr-prep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mrURL := fs.String("url", "", "gitlab merge request url")
	repo := fs.String("repo", ".", "candidate local repo (cloned if it is not the MR project)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mrURL == "" {
		fmt.Fprintln(stderr, "usage: ccr-engine mr-prep --url <gitlab-mr-url> [--repo dir]")
		return 2
	}

	res, err := mrprep.Prep(*mrURL, *repo)
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
