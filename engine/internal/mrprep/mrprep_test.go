package mrprep

import (
	"encoding/json"
	"testing"
)

func TestMRAPIDecode(t *testing.T) {
	payload := `{"iid":42,"target_branch":"main","source_branch":"feat","sha":"deadbeef",
		"diff_refs":{"base_sha":"base1","head_sha":"head1","start_sha":"start1"}}`
	var m mrAPI
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatal(err)
	}
	if m.IID != 42 || m.TargetBranch != "main" {
		t.Fatalf("meta=%+v", m)
	}
	if m.DiffRefs.BaseSHA != "base1" || m.DiffRefs.HeadSHA != "head1" || m.DiffRefs.StartSHA != "start1" {
		t.Fatalf("diff_refs=%+v", m.DiffRefs)
	}
}
