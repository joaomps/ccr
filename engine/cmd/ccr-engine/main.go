// Command ccr-engine is the deterministic core of the ccr code-review tool.
// It owns the correctness-critical steps: file selection, rule matching,
// diff line numbering, finding validation/positioning, and rendering.
// Judgment (finding bugs, scoring confidence) is left to Claude Code subagents
// driven by the /ccr:review slash command.
package main

import (
	"fmt"
	"io"
	"os"
)

const version = "0.1.0"

const usage = "usage: ccr-engine <plan|collect|report|rules|mr-prep|pr-prep|version> [flags]"

type command func(args []string, stdout, stderr io.Writer) int

// commands returns the subcommand dispatch table. Subcommands are added as
// later phases land; version is always present.
func commands() map[string]command {
	return map[string]command{
		"version": cmdVersion,
		"rules":   cmdRules,
		"plan":    cmdPlan,
		"collect": cmdCollect,
		"report":  cmdReport,
		"mr-prep": cmdMRPrep,
		"pr-prep": cmdPRPrep,
	}
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, version)
	return 0
}

// Run dispatches a subcommand and returns the process exit code. It is the
// testable entry point; main only wires it to the real process streams.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, usage)
		return 2
	}
	cmd, ok := commands()[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "unknown subcommand %q\n%s\n", args[0], usage)
		return 2
	}
	return cmd(args[1:], stdout, stderr)
}

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}
