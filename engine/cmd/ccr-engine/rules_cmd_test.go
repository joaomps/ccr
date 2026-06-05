package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesCheckProject(t *testing.T) {
	repo := t.TempDir()
	t.Setenv("HOME", t.TempDir()) // isolate from any real ~/.ccr
	ccr := filepath.Join(repo, ".ccr")
	if err := os.MkdirAll(ccr, 0o755); err != nil {
		t.Fatal(err)
	}
	proj := `{"layers":[{"path":"**/*.go","rules":[{"id":"proj-go","severity":"high","category":"x","guidance":"g"}]}]}`
	if err := os.WriteFile(filepath.Join(ccr, "rule.json"), []byte(proj), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := Run([]string{"rules", "check", "--repo", repo, "a.go"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	got := out.String()
	if !strings.Contains(got, "proj-go") || !strings.Contains(got, "source: project") {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestRulesCheckCatchAll(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out, errb bytes.Buffer
	code := Run([]string{"rules", "check", "--repo", t.TempDir(), "config.weirdext"}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "generic-correctness") {
		t.Fatalf("expected generic-correctness, got %s", out.String())
	}
}

func TestRulesCheckMissingFile(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"rules", "check", "--repo", t.TempDir()}, &out, &errb); code == 0 {
		t.Fatal("expected non-zero exit when file missing")
	}
}
