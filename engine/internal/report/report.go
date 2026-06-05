// Package report renders positioned (and optionally reflected) findings into a
// human or machine readable review. It filters by confidence and never modifies
// source files.
package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ccr/internal/model"
)

// Render produces the review in the requested format: "md" (default), "text",
// or "json". A finding with a positive Confidence below minConfidence is
// filtered out; Confidence == 0 means the reflector did not run and the finding
// is kept.
func Render(p model.Positioned, minConfidence float64, format string) (string, error) {
	kept := filter(p.Findings, minConfidence)
	sortFindings(kept)

	switch format {
	case "json":
		out := model.Positioned{RunID: p.RunID, Findings: kept, Dropped: p.Dropped}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	case "text":
		return renderText(kept, p.Dropped), nil
	default:
		return renderMarkdown(kept, p.Dropped), nil
	}
}

func filter(in []model.PositionedFinding, min float64) []model.PositionedFinding {
	out := make([]model.PositionedFinding, 0, len(in))
	for _, f := range in {
		if f.Confidence > 0 && f.Confidence < min {
			continue
		}
		out = append(out, f)
	}
	return out
}

func counts(f []model.PositionedFinding) (high, med, low int) {
	for _, x := range f {
		switch x.Severity {
		case "high":
			high++
		case "low":
			low++
		default:
			med++
		}
	}
	return
}

func sevRank(s string) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	}
	return 0
}

func sortFindings(f []model.PositionedFinding) {
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].File != f[j].File {
			return f[i].File < f[j].File
		}
		if f[i].Severity != f[j].Severity {
			return sevRank(f[i].Severity) > sevRank(f[j].Severity)
		}
		return f[i].Line < f[j].Line
	})
}

func renderMarkdown(f []model.PositionedFinding, dropped []model.Dropped) string {
	var b strings.Builder
	high, med, low := counts(f)
	fmt.Fprintf(&b, "# Code Review\n\n%d findings — %d high, %d medium, %d low. %d dropped.\n",
		len(f), high, med, low, len(dropped))
	if len(f) == 0 {
		b.WriteString("\nNo issues found.\n")
		return b.String()
	}
	curFile := ""
	for _, x := range f {
		if x.File != curFile {
			curFile = x.File
			fmt.Fprintf(&b, "\n## %s\n", curFile)
		}
		approx := ""
		if x.Anchor == "hunk-fallback" {
			approx = "  _(approx)_"
		}
		fmt.Fprintf(&b, "\n### %s:%d — %s — %s  `%s`%s\n",
			x.File, x.Line, strings.ToUpper(x.Severity), x.Title, x.RuleID, approx)
		if r := strings.TrimSpace(x.Rationale); r != "" {
			fmt.Fprintf(&b, "%s\n", r)
		}
		if fix := strings.TrimRight(x.SuggestedFix, "\n"); strings.TrimSpace(fix) != "" {
			fmt.Fprintf(&b, "\n```diff\n%s\n```\n", fix)
		}
	}
	return b.String()
}

func renderText(f []model.PositionedFinding, dropped []model.Dropped) string {
	var b strings.Builder
	high, med, low := counts(f)
	fmt.Fprintf(&b, "Code Review: %d findings (%d high, %d medium, %d low), %d dropped\n",
		len(f), high, med, low, len(dropped))
	for _, x := range f {
		approx := ""
		if x.Anchor == "hunk-fallback" {
			approx = " (approx)"
		}
		fmt.Fprintf(&b, "\n%s:%d  %s  %s (%s)%s\n",
			x.File, x.Line, strings.ToUpper(x.Severity), x.Title, x.RuleID, approx)
		if r := strings.TrimSpace(x.Rationale); r != "" {
			fmt.Fprintf(&b, "    %s\n", r)
		}
		if fix := strings.TrimRight(x.SuggestedFix, "\n"); strings.TrimSpace(fix) != "" {
			b.WriteString("    fix:\n")
			for _, ln := range strings.Split(fix, "\n") {
				fmt.Fprintf(&b, "      %s\n", ln)
			}
		}
	}
	return b.String()
}
