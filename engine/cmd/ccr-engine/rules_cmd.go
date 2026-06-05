package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"ccr/internal/rules"
)

// cmdRules handles: ccr-engine rules check [--repo dir] [--rule path] <file>
// Flags are expected before the positional <file> (Go's flag package stops at
// the first non-flag argument).
func cmdRules(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: ccr-engine rules check [--repo dir] [--rule path] <file>")
		return 2
	}
	fs := flag.NewFlagSet("rules check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	repo := fs.String("repo", ".", "git repository root")
	rulePath := fs.String("rule", "", "path to custom rule json")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	file := fs.Arg(0)
	if file == "" {
		fmt.Fprintln(stderr, "rules check: missing <file>")
		return 2
	}

	home, _ := os.UserHomeDir()
	res, err := rules.Resolve(*repo, *rulePath, home)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	rs, src, ok := res.RulesForWithSource(file)
	if !ok {
		fmt.Fprintf(stdout, "%s -> [] (no match)\n", file)
		return 0
	}
	ids := make([]string, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	fmt.Fprintf(stdout, "%s -> [%s] (source: %s)\n", file, strings.Join(ids, " "), src)
	return 0
}
