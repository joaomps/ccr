package collect

import (
	"testing"

	"ccr/internal/model"
)

func bundleAB() model.Bundle {
	return model.Bundle{
		ID:    "b001",
		Files: []string{"a.go"},
		ReviewLines: map[string][]model.ReviewLine{
			"a.go": {{Line: 42, Code: "x := foo()"}, {Line: 43, Code: "return x.Bar()"}},
		},
	}
}

func TestPositionExact(t *testing.T) {
	pf, reason := positionOne(model.RawFinding{File: "a.go", Line: 43, Title: "t", Severity: "high"}, bundleAB())
	if reason != "" || pf.Anchor != "exact" || pf.Line != 43 {
		t.Fatalf("pf=%+v reason=%q", pf, reason)
	}
}

func TestPositionRecoveredByCode(t *testing.T) {
	// wrong line, but cited code (extra spaces) matches line 43 after normalization
	pf, reason := positionOne(model.RawFinding{File: "a.go", Line: 999, Code: "return  x.Bar()", Title: "t"}, bundleAB())
	if reason != "" || pf.Anchor != "exact-recovered" || pf.Line != 43 {
		t.Fatalf("pf=%+v reason=%q", pf, reason)
	}
}

func TestPositionHunkFallback(t *testing.T) {
	pf, reason := positionOne(model.RawFinding{File: "a.go", Line: 999, Title: "t"}, bundleAB())
	if reason != "" || pf.Anchor != "hunk-fallback" || pf.Line != 43 {
		t.Fatalf("pf=%+v reason=%q", pf, reason)
	}
}

func TestPositionFileNotInBundle(t *testing.T) {
	_, reason := positionOne(model.RawFinding{File: "z.go", Line: 1, Title: "t"}, bundleAB())
	if reason != "file_not_in_bundle" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestPositionRecoverAcrossFile(t *testing.T) {
	// names the wrong file but cites code that exists in a.go
	pf, reason := positionOne(model.RawFinding{File: "z.go", Line: 1, Code: "x := foo()", Title: "t"}, bundleAB())
	if reason != "" || pf.File != "a.go" || pf.Line != 42 || pf.Anchor != "exact-recovered" {
		t.Fatalf("pf=%+v reason=%q", pf, reason)
	}
}

func TestNormalizeSeverity(t *testing.T) {
	if normalizeSeverity("HIGH") != "high" {
		t.Fatal("HIGH should normalize to high")
	}
	if normalizeSeverity("") != "medium" {
		t.Fatal("empty should default to medium")
	}
	if normalizeSeverity("bogus") != "medium" {
		t.Fatal("unknown should clamp to medium")
	}
}
