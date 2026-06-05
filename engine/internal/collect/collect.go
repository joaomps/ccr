package collect

import (
	"fmt"
	"sort"
	"strings"

	"ccr/internal/model"
)

// Collect positions every raw finding against its bundle, deduplicates, assigns
// stable IDs, and returns the positioned result. Findings that cannot be
// validated or placed are recorded in Dropped (never silently discarded).
func Collect(plan model.Plan, findingsDir string) (model.Positioned, error) {
	bundleByID := make(map[string]model.Bundle, len(plan.Bundles))
	for _, b := range plan.Bundles {
		bundleByID[b.ID] = b
	}

	bundleFindings, fileDropped, err := LoadFindings(findingsDir)
	if err != nil {
		return model.Positioned{}, err
	}

	positioned := []model.PositionedFinding{}
	dropped := append([]model.Dropped(nil), fileDropped...)

	for _, bf := range bundleFindings {
		b, ok := bundleByID[bf.BundleID]
		if !ok {
			for _, rf := range bf.Findings {
				dropped = append(dropped, model.Dropped{Reason: "unknown_bundle:" + bf.BundleID, Raw: rf})
			}
			continue
		}
		for _, rf := range bf.Findings {
			if strings.TrimSpace(rf.File) == "" || strings.TrimSpace(rf.Title) == "" {
				dropped = append(dropped, model.Dropped{Reason: "schema_invalid", Raw: rf})
				continue
			}
			pf, reason := positionOne(rf, b)
			if reason != "" {
				dropped = append(dropped, model.Dropped{Reason: reason, Raw: rf})
				continue
			}
			positioned = append(positioned, pf)
		}
	}

	positioned = dedup(positioned)
	sortFindings(positioned)
	for i := range positioned {
		positioned[i].ID = fmt.Sprintf("f%03d", i+1)
	}

	return model.Positioned{RunID: plan.RunID, Findings: positioned, Dropped: dropped}, nil
}

type dedupKey struct {
	file string
	line int
	rule string
}

// dedup collapses findings sharing (file, line, rule_id), keeping the highest
// severity and merging distinct rationales.
func dedup(in []model.PositionedFinding) []model.PositionedFinding {
	idx := map[dedupKey]int{}
	var out []model.PositionedFinding
	for _, f := range in {
		k := dedupKey{f.File, f.Line, f.RuleID}
		if j, ok := idx[k]; ok {
			if sevRank(f.Severity) > sevRank(out[j].Severity) {
				out[j].Severity = f.Severity
				out[j].Title = f.Title
			}
			if f.Rationale != "" && !strings.Contains(out[j].Rationale, f.Rationale) {
				out[j].Rationale = strings.TrimSpace(out[j].Rationale + " " + f.Rationale)
			}
			if out[j].SuggestedFix == "" {
				out[j].SuggestedFix = f.SuggestedFix
			}
			continue
		}
		idx[k] = len(out)
		out = append(out, f)
	}
	return out
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
