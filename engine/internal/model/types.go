// Package model defines the JSON contracts exchanged between the engine
// subcommands and the Claude Code subagents. Field names are the wire format
// and must not change without updating the plugin prompts that produce/consume
// them.
package model

// Rule is a single review rule matched to a file by glob.
type Rule struct {
	ID       string `json:"id"`
	Severity string `json:"severity"` // high|medium|low
	Category string `json:"category"`
	Guidance string `json:"guidance"`
}

// ReviewLine is one reviewable (added/changed) line, numbered by absolute
// new-file line. The file-reviewer subagent must cite findings by choosing a
// Line from this set.
type ReviewLine struct {
	Line int    `json:"line"`
	Code string `json:"code"`
}

// Bundle is a unit of review: related files grouped together, the matched
// rules, the bundle's diff text, and the per-file menu of reviewable lines.
type Bundle struct {
	ID          string                  `json:"id"`
	Files       []string                `json:"files"`
	Rules       []Rule                  `json:"rules"`
	Diff        string                  `json:"diff"`
	ReviewLines map[string][]ReviewLine `json:"review_lines"`
}

// Skipped records a file excluded from review and why.
type Skipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// DiffRefs are the GitLab MR diff anchors needed for inline comment posting
// (captured even when v1 only prints to the terminal).
type DiffRefs struct {
	BaseSHA  string `json:"base_sha"`
	HeadSHA  string `json:"head_sha"`
	StartSHA string `json:"start_sha"`
}

// MRInfo describes a GitLab merge request resolved by mr-prep.
type MRInfo struct {
	URL      string   `json:"url"`
	IID      int      `json:"iid"`
	Project  string   `json:"project"`
	Host     string   `json:"host"`
	DiffRefs DiffRefs `json:"diff_refs"`
}

// Plan is the output of the plan subcommand: the full deterministic review plan.
type Plan struct {
	RunID   string    `json:"run_id"`
	Mode    string    `json:"mode"` // worktree|range|commit|mr
	Repo    string    `json:"repo"`
	BaseRef string    `json:"base_ref"`
	HeadRef string    `json:"head_ref"`
	MR      *MRInfo   `json:"mr"`
	Bundles []Bundle  `json:"bundles"`
	Skipped []Skipped `json:"skipped"`
}

// RawFinding is produced by the file-reviewer subagent. It is untrusted input:
// the engine validates and repairs it in collect.
type RawFinding struct {
	File         string `json:"file"`
	Line         int    `json:"line"`
	RuleID       string `json:"rule_id"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Rationale    string `json:"rationale"`
	SuggestedFix string `json:"suggested_fix"`
}

// BundleFindings is the file a file-reviewer subagent writes per bundle.
type BundleFindings struct {
	BundleID string       `json:"bundle_id"`
	Findings []RawFinding `json:"findings"`
}

// PositionedFinding is a validated, line-anchored finding after collect.
// Anchor is one of: exact, exact-recovered, hunk-fallback.
type PositionedFinding struct {
	ID           string  `json:"id"`
	File         string  `json:"file"`
	Line         int     `json:"line"`
	Anchor       string  `json:"anchor"`
	RuleID       string  `json:"rule_id"`
	Severity     string  `json:"severity"`
	Title        string  `json:"title"`
	Rationale    string  `json:"rationale"`
	SuggestedFix string  `json:"suggested_fix"`
	Confidence   float64 `json:"confidence,omitempty"` // set by the reflector subagent
}

// Dropped records a raw finding that could not be positioned, and why.
type Dropped struct {
	Reason string     `json:"reason"`
	Raw    RawFinding `json:"raw"`
}

// Positioned is the output of the collect subcommand.
type Positioned struct {
	RunID    string              `json:"run_id"`
	Findings []PositionedFinding `json:"findings"`
	Dropped  []Dropped           `json:"dropped"`
}
