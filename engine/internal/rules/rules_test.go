package rules

import (
	"os"
	"path/filepath"
	"testing"

	"ccr/internal/model"
)

func ruleIDs(rs []model.Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestMatchFileFirstWins(t *testing.T) {
	rf := RuleFile{Layers: []LayerEntry{
		{Path: "**/*_test.go", Rules: []model.Rule{{ID: "test-rule"}}},
		{Path: "**/*.go", Rules: []model.Rule{{ID: "go-rule"}}},
	}}
	rs, ok := rf.MatchFile("a_test.go")
	if !ok || len(rs) != 1 || rs[0].ID != "test-rule" {
		t.Fatalf("expected test-rule first, got %v ok=%v", ruleIDs(rs), ok)
	}
	rs, ok = rf.MatchFile("a.go")
	if !ok || rs[0].ID != "go-rule" {
		t.Fatalf("expected go-rule, got %v", ruleIDs(rs))
	}
}

func TestMatchFileEmptyRulesStillCovers(t *testing.T) {
	rf := RuleFile{Layers: []LayerEntry{
		{Path: "**/*.md", Rules: []model.Rule{}},
		{Path: "**/*", Rules: []model.Rule{{ID: "catch"}}},
	}}
	rs, ok := rf.MatchFile("README.md")
	if !ok {
		t.Fatal("expected md to be covered")
	}
	if len(rs) != 0 {
		t.Fatalf("expected empty rules, got %v", ruleIDs(rs))
	}
}

func TestMatchFileNoMatch(t *testing.T) {
	rf := RuleFile{Layers: []LayerEntry{{Path: "**/*.go", Rules: nil}}}
	if _, ok := rf.MatchFile("a.ts"); ok {
		t.Fatal("expected no match for .ts")
	}
}

func TestResolveEmbeddedDefaults(t *testing.T) {
	repo := t.TempDir()
	res, err := Resolve(repo, "", "")
	if err != nil {
		t.Fatal(err)
	}
	rs, src, ok := res.RulesForWithSource("internal/svc/a.go")
	if !ok || src != "embedded" {
		t.Fatalf("want embedded source, got src=%q ok=%v", src, ok)
	}
	if !contains(ruleIDs(rs), "go-nil-deref") {
		t.Fatalf("expected go-nil-deref in embedded .go rules, got %v", ruleIDs(rs))
	}
	// Unknown extension hits the generic catch-all.
	rs, _, ok = res.RulesForWithSource("config.weirdext")
	if !ok || !contains(ruleIDs(rs), "generic-correctness") {
		t.Fatalf("expected generic-correctness catch-all, got %v ok=%v", ruleIDs(rs), ok)
	}
}

func TestResolveProjectOverrides(t *testing.T) {
	repo := t.TempDir()
	ccr := filepath.Join(repo, ".ccr")
	if err := os.MkdirAll(ccr, 0o755); err != nil {
		t.Fatal(err)
	}
	proj := `{"layers":[{"path":"**/*.go","rules":[{"id":"proj-go","severity":"high","category":"x","guidance":"g"}]}]}`
	if err := os.WriteFile(filepath.Join(ccr, "rule.json"), []byte(proj), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Resolve(repo, "", "")
	if err != nil {
		t.Fatal(err)
	}
	rs, src, _ := res.RulesForWithSource("a.go")
	if src != "project" || !contains(ruleIDs(rs), "proj-go") {
		t.Fatalf("expected project proj-go, got src=%q rules=%v", src, ruleIDs(rs))
	}
}

func TestResolveMissingFilesSkip(t *testing.T) {
	// repo and home with no .ccr/rule.json must not error.
	if _, err := Resolve(t.TempDir(), "", t.TempDir()); err != nil {
		t.Fatalf("missing optional files should not error: %v", err)
	}
}

func TestResolveMalformedRuleFlagErrors(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(t.TempDir(), bad, ""); err == nil {
		t.Fatal("expected error for malformed --rule file")
	}
}
