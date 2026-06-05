package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"ccr/internal/collect"
	"ccr/internal/model"
)

// cmdCollect handles: ccr-engine collect --plan plan.json --findings-dir dir [--run-id id]
func cmdCollect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	planPath := fs.String("plan", "", "path to plan.json")
	findingsDir := fs.String("findings-dir", "", "directory of bundle findings json")
	runID := fs.String("run-id", "", "override the plan run id")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *planPath == "" || *findingsDir == "" {
		fmt.Fprintln(stderr, "usage: ccr-engine collect --plan plan.json --findings-dir dir [--run-id id]")
		return 2
	}

	b, err := os.ReadFile(*planPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var plan model.Plan
	if err := json.Unmarshal(b, &plan); err != nil {
		fmt.Fprintf(stderr, "parsing plan: %v\n", err)
		return 1
	}
	if *runID != "" {
		plan.RunID = *runID
	}

	pos, err := collect.Collect(plan, *findingsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(pos); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}
