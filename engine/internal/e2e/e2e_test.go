// Package e2e exercises the full deterministic CLI chain (plan -> collect ->
// report) against the real built binary, with a hand-written findings file
// standing in for the file-reviewer subagent.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ccr/internal/model"
)

func buildEngine(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ccr-engine")
	cmd := exec.Command("go", "build", "-o", bin, "ccr/cmd/ccr-engine")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building engine: %v", err)
	}
	return bin
}

func git(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func run(t *testing.T, bin string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v: %v\nstderr: %s", bin, args, err, errb.String())
	}
	return out.Bytes()
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEndToEndCLIChain(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	bin := buildEngine(t)
	repo := t.TempDir()

	git(t, repo, "init", "-q")
	src := filepath.Join(repo, "svc", "a.go")
	writeFile(t, src, "package svc\n\nfunc A() int {\n\treturn 1\n}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-qm", "baseline")
	// Plant a nil-pointer dereference (uncommitted -> worktree mode sees it).
	writeFile(t, src, "package svc\n\nfunc A() int {\n\tvar p *int\n\treturn *p\n}\n")

	// 1. plan
	planOut := run(t, bin, "plan", "--repo", repo, "--run-id", "e2e")
	var plan model.Plan
	if err := json.Unmarshal(planOut, &plan); err != nil {
		t.Fatalf("plan json: %v", err)
	}
	if len(plan.Bundles) == 0 {
		t.Fatalf("expected a bundle, got plan: %s", planOut)
	}
	b := plan.Bundles[0]
	file := b.Files[0]

	// Find the planted line in the review-line menu.
	var line int
	var code string
	for _, rl := range b.ReviewLines[file] {
		if strings.Contains(rl.Code, "return *p") {
			line, code = rl.Line, rl.Code
		}
	}
	if line == 0 {
		t.Fatalf("planted line not in review_lines: %+v", b.ReviewLines)
	}

	// 2. write a findings file (stands in for the file-reviewer subagent)
	base := filepath.Join(repo, ".ccr", "tmp", "e2e")
	findingsDir := filepath.Join(base, "findings")
	bf := model.BundleFindings{
		BundleID: b.ID,
		Findings: []model.RawFinding{{
			File: file, Line: line, Code: code,
			RuleID: "go-nil-deref", Severity: "high",
			Title: "nil pointer dereference", Rationale: "p is always nil here",
			SuggestedFix: "return 0",
		}},
	}
	fb, _ := json.Marshal(bf)
	writeFile(t, filepath.Join(findingsDir, b.ID+".json"), string(fb))

	planPath := filepath.Join(base, "plan.json")
	writeFile(t, planPath, string(planOut))

	// 3. collect
	posOut := run(t, bin, "collect", "--plan", planPath, "--findings-dir", findingsDir, "--run-id", "e2e")
	posPath := filepath.Join(base, "positioned.json")
	writeFile(t, posPath, string(posOut))

	// 4. report
	rep := string(run(t, bin, "report", "--reflected", posPath, "--format", "md"))

	for _, want := range []string{file, "HIGH", "nil pointer dereference", fmt.Sprintf(":%d", line)} {
		if !strings.Contains(rep, want) {
			t.Fatalf("report missing %q:\n%s", want, rep)
		}
	}
}
