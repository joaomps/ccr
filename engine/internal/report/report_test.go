package report

import (
	"encoding/json"
	"strings"
	"testing"

	"ccr/internal/model"
)

func sample() model.Positioned {
	return model.Positioned{
		RunID: "r1",
		Findings: []model.PositionedFinding{
			{ID: "f001", File: "a.go", Line: 5, Anchor: "exact", RuleID: "r1", Severity: "medium", Title: "med", Rationale: "because"},
			{ID: "f002", File: "a.go", Line: 9, Anchor: "hunk-fallback", RuleID: "r2", Severity: "high", Title: "hi", SuggestedFix: "- bad\n+ good\n"},
		},
	}
}

func TestRenderMarkdownOrderAndApprox(t *testing.T) {
	out, err := Render(sample(), 0.5, "md")
	if err != nil {
		t.Fatal(err)
	}
	hi := strings.Index(out, "HIGH")
	med := strings.Index(out, "MEDIUM")
	if hi < 0 || med < 0 || hi > med {
		t.Fatalf("high should sort before medium:\n%s", out)
	}
	if !strings.Contains(out, "_(approx)_") {
		t.Fatalf("missing approx label:\n%s", out)
	}
	if !strings.Contains(out, "```diff") {
		t.Fatalf("missing diff block:\n%s", out)
	}
}

func TestRenderConfidenceFilter(t *testing.T) {
	p := sample()
	p.Findings[0].Confidence = 0.2 // below threshold -> filtered
	p.Findings[1].Confidence = 0.9 // kept
	out, _ := Render(p, 0.5, "md")
	if strings.Contains(out, "because") {
		t.Fatalf("low-confidence finding should be filtered:\n%s", out)
	}
	if !strings.Contains(out, "`r2`") {
		t.Fatalf("high-confidence finding should remain:\n%s", out)
	}
}

func TestRenderConfidenceZeroKept(t *testing.T) {
	out, _ := Render(sample(), 0.5, "md")
	if !strings.Contains(out, "because") || !strings.Contains(out, "`r2`") {
		t.Fatalf("zero-confidence findings should be kept:\n%s", out)
	}
}

func TestRenderEmpty(t *testing.T) {
	out, _ := Render(model.Positioned{RunID: "r1"}, 0.5, "md")
	if !strings.Contains(out, "No issues found") {
		t.Fatalf("empty render should say no issues:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	out, err := Render(sample(), 0.5, "json")
	if err != nil {
		t.Fatal(err)
	}
	var back model.Positioned
	if err := json.Unmarshal([]byte(out), &back); err != nil {
		t.Fatalf("json invalid: %v", err)
	}
	if len(back.Findings) != 2 {
		t.Fatalf("want 2, got %d", len(back.Findings))
	}
}
