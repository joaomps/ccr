package collect

import (
	"testing"

	"ccr/internal/model"
)

func TestCollectDedupAndOrder(t *testing.T) {
	plan := model.Plan{
		RunID: "r1",
		Bundles: []model.Bundle{{
			ID:    "b001",
			Files: []string{"a.go"},
			ReviewLines: map[string][]model.ReviewLine{
				"a.go": {{Line: 10, Code: "a"}, {Line: 20, Code: "b"}},
			},
		}},
	}
	dir := t.TempDir()
	writeJSON(t, dir, "b001.json", `{"bundle_id":"b001","findings":[
		{"file":"a.go","line":10,"rule_id":"r","severity":"medium","title":"dup","rationale":"one"},
		{"file":"a.go","line":10,"rule_id":"r","severity":"high","title":"dup","rationale":"two"},
		{"file":"a.go","line":20,"rule_id":"q","severity":"low","title":"other"}
	]}`)

	pos, err := Collect(plan, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos.Findings) != 2 {
		t.Fatalf("want 2 after dedup, got %d: %+v", len(pos.Findings), pos.Findings)
	}
	if pos.Findings[0].Severity != "high" || pos.Findings[0].Line != 10 {
		t.Fatalf("expected high@10 first, got %+v", pos.Findings[0])
	}
	if pos.Findings[0].ID != "f001" || pos.Findings[1].ID != "f002" {
		t.Fatalf("ids not stable: %s %s", pos.Findings[0].ID, pos.Findings[1].ID)
	}
}

func TestCollectUnknownBundleDropped(t *testing.T) {
	plan := model.Plan{RunID: "r1", Bundles: []model.Bundle{{ID: "b001"}}}
	dir := t.TempDir()
	writeJSON(t, dir, "x.json", `{"bundle_id":"bZZZ","findings":[{"file":"a.go","line":1,"title":"t"}]}`)
	pos, err := Collect(plan, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %+v", pos.Findings)
	}
	if len(pos.Dropped) != 1 || pos.Dropped[0].Reason != "unknown_bundle:bZZZ" {
		t.Fatalf("dropped=%+v", pos.Dropped)
	}
}

func TestCollectSchemaInvalidDropped(t *testing.T) {
	plan := model.Plan{
		RunID: "r1",
		Bundles: []model.Bundle{{
			ID:          "b001",
			Files:       []string{"a.go"},
			ReviewLines: map[string][]model.ReviewLine{"a.go": {{Line: 1, Code: "x"}}},
		}},
	}
	dir := t.TempDir()
	writeJSON(t, dir, "b001.json", `{"bundle_id":"b001","findings":[
		{"file":"","line":1,"title":"no file"},
		{"file":"a.go","line":1,"title":""}
	]}`)
	pos, err := Collect(plan, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(pos.Findings) != 0 {
		t.Fatalf("findings=%+v", pos.Findings)
	}
	if len(pos.Dropped) != 2 {
		t.Fatalf("expected 2 dropped, got %+v", pos.Dropped)
	}
}
