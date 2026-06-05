package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"ccr/internal/model"
	"ccr/internal/report"
)

// cmdReport handles: ccr-engine report --reflected file.json [--min-confidence 0.5] [--format md|text|json]
// The input file is a Positioned document; reflected.json (with confidence) and
// positioned.json (without) are both accepted.
func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	input := fs.String("reflected", "", "path to reflected/positioned json")
	minConf := fs.Float64("min-confidence", 0.5, "drop findings below this confidence (0 disables)")
	format := fs.String("format", "md", "output format: md|text|json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *input == "" {
		fmt.Fprintln(stderr, "usage: ccr-engine report --reflected file.json [--min-confidence 0.5] [--format md|text|json]")
		return 2
	}

	b, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var pos model.Positioned
	if err := json.Unmarshal(b, &pos); err != nil {
		fmt.Fprintf(stderr, "parsing %s: %v\n", *input, err)
		return 1
	}

	out, err := report.Render(pos, *minConf, *format)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}
