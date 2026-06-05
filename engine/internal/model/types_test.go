package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlanRoundTrip(t *testing.T) {
	p := Plan{
		RunID:   "r1",
		Mode:    "worktree",
		Repo:    "/tmp/repo",
		BaseRef: "HEAD",
		HeadRef: "WORKTREE",
		MR:      nil,
		Bundles: []Bundle{{
			ID:    "b001",
			Files: []string{"a.go"},
			Rules: []Rule{{ID: "go-nil-deref", Severity: "high", Category: "correctness", Guidance: "g"}},
			Diff:  "@@ -1 +1 @@",
			ReviewLines: map[string][]ReviewLine{
				"a.go": {{Line: 42, Code: "x := foo()"}},
			},
		}},
		Skipped: []Skipped{{File: "vendor/x.go", Reason: "ignored:**/vendor/**"}},
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	// Wire-format key assertions.
	for _, key := range []string{`"run_id"`, `"review_lines"`, `"base_ref"`, `"head_ref"`} {
		if !strings.Contains(s, key) {
			t.Errorf("missing key %s in %s", key, s)
		}
	}
	// nil *MRInfo serializes to null.
	if !strings.Contains(s, `"mr":null`) {
		t.Errorf("expected mr:null, got %s", s)
	}

	var back Plan
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Bundles[0].ReviewLines["a.go"][0].Line != 42 {
		t.Errorf("round-trip lost review line: %+v", back)
	}
}

func TestRawFindingKeys(t *testing.T) {
	b, _ := json.Marshal(RawFinding{File: "a.go", Line: 1, RuleID: "x", Severity: "high", Title: "t"})
	for _, key := range []string{`"rule_id"`, `"suggested_fix"`} {
		if !strings.Contains(string(b), key) {
			t.Errorf("missing key %s in %s", key, b)
		}
	}
}
