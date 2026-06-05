package collect

import (
	"strings"

	"ccr/internal/model"
)

// positionOne resolves a raw finding to a concrete line within bundle b using
// constrained-choice validation:
//
//  1. if the cited line is one of the bundle's reviewable lines -> exact
//  2. else if the cited code matches a reviewable line          -> exact-recovered
//  3. else                                                       -> hunk-fallback (approximate)
//
// If the finding names a file outside the bundle, recovery is attempted across
// all of the bundle's files; failing that the finding is dropped.
// On success returns (finding, ""); on drop returns (zero, reason).
func positionOne(rf model.RawFinding, b model.Bundle) (model.PositionedFinding, string) {
	lines, inBundle := b.ReviewLines[rf.File]
	if !inBundle {
		if f, line, ok := recoverBySnippet(rf, b); ok {
			return finalize(rf, f, line, "exact-recovered"), ""
		}
		return model.PositionedFinding{}, "file_not_in_bundle"
	}
	if lineInSet(rf.Line, lines) {
		return finalize(rf, rf.File, rf.Line, "exact"), ""
	}
	if line, ok := matchSnippet(citedCode(rf), lines); ok {
		return finalize(rf, rf.File, line, "exact-recovered"), ""
	}
	return finalize(rf, rf.File, nearestReviewLine(rf.Line, lines), "hunk-fallback"), ""
}

func finalize(rf model.RawFinding, file string, line int, anchor string) model.PositionedFinding {
	return model.PositionedFinding{
		File:         file,
		Line:         line,
		Anchor:       anchor,
		RuleID:       rf.RuleID,
		Severity:     normalizeSeverity(rf.Severity),
		Title:        rf.Title,
		Rationale:    rf.Rationale,
		SuggestedFix: rf.SuggestedFix,
	}
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}

// citedCode returns the offending line the finding pointed at: the explicit
// code field if present, otherwise the first non-empty line of the suggested fix.
func citedCode(rf model.RawFinding) string {
	if strings.TrimSpace(rf.Code) != "" {
		return rf.Code
	}
	for _, ln := range strings.Split(rf.SuggestedFix, "\n") {
		if strings.TrimSpace(ln) != "" {
			return ln
		}
	}
	return ""
}

func lineInSet(line int, lines []model.ReviewLine) bool {
	for _, l := range lines {
		if l.Line == line {
			return true
		}
	}
	return false
}

func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// matchSnippet returns the line of the unique reviewable line whose code matches
// the snippet after whitespace normalization. A non-unique or empty match fails.
func matchSnippet(snippet string, lines []model.ReviewLine) (int, bool) {
	want := normalize(snippet)
	if want == "" {
		return 0, false
	}
	line, count := 0, 0
	for _, l := range lines {
		if normalize(l.Code) == want {
			line, count = l.Line, count+1
		}
	}
	if count == 1 {
		return line, true
	}
	return 0, false
}

func recoverBySnippet(rf model.RawFinding, b model.Bundle) (file string, line int, found bool) {
	if normalize(citedCode(rf)) == "" {
		return "", 0, false
	}
	for _, f := range b.Files {
		if l, ok := matchSnippet(citedCode(rf), b.ReviewLines[f]); ok {
			return f, l, true
		}
	}
	return "", 0, false
}

// nearestReviewLine returns the reviewable line closest to target (the
// approximate fallback anchor). Returns 0 if the file has no reviewable lines.
func nearestReviewLine(target int, lines []model.ReviewLine) int {
	if len(lines) == 0 {
		return 0
	}
	best, bestDist := lines[0].Line, abs(lines[0].Line-target)
	for _, l := range lines[1:] {
		if d := abs(l.Line - target); d < bestDist {
			best, bestDist = l.Line, d
		}
	}
	return best
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
