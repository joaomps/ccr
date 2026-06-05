package plan

import "testing"

func TestSelect(t *testing.T) {
	files := []string{
		"internal/svc/a.go",
		"vendor/x/y.go",
		"go.sum",
		"web/app.min.js",
		"api/api.pb.go",
		"cmd/main.go",
	}
	kept, skipped := Select(files)

	keptSet := map[string]bool{}
	for _, k := range kept {
		keptSet[k] = true
	}
	if !keptSet["internal/svc/a.go"] || !keptSet["cmd/main.go"] {
		t.Fatalf("expected source files kept, got %v", kept)
	}
	if keptSet["vendor/x/y.go"] || keptSet["go.sum"] || keptSet["api/api.pb.go"] {
		t.Fatalf("expected generated/vendored skipped, kept=%v", kept)
	}
	skippedSet := map[string]string{}
	for _, s := range skipped {
		skippedSet[s.File] = s.Reason
	}
	if skippedSet["vendor/x/y.go"] == "" || skippedSet["go.sum"] == "" {
		t.Fatalf("expected skip reasons, got %v", skipped)
	}
}
