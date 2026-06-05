package plan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T) (string, func(args ...string)) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	return repo, run
}

func writeFile(t *testing.T, repo, name, content string) {
	t.Helper()
	p := filepath.Join(repo, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildWorktree(t *testing.T) {
	repo, run := gitRepo(t)
	writeFile(t, repo, "svc/a.go", "package svc\nfunc A() {}\n")
	writeFile(t, repo, "api/b.go", "package api\nfunc B() {}\n")
	run("add", ".")
	run("commit", "-qm", "init")
	writeFile(t, repo, "svc/a.go", "package svc\nfunc A() { println(1) }\n")
	writeFile(t, repo, "api/b.go", "package api\nfunc B() { println(2) }\n")

	p, err := Build(Options{Repo: repo, Mode: "worktree", HomeDir: t.TempDir(), RunID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Mode != "worktree" || p.RunID != "r1" {
		t.Fatalf("plan meta: %+v", p)
	}
	if len(p.Bundles) != 2 {
		t.Fatalf("want 2 bundles, got %d: %+v", len(p.Bundles), p.Bundles)
	}
	for _, b := range p.Bundles {
		f := b.Files[0]
		if len(b.ReviewLines[f]) == 0 {
			t.Fatalf("no review lines for %s", f)
		}
		found := false
		for _, r := range b.Rules {
			if r.ID == "go-nil-deref" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected go-nil-deref in bundle rules: %+v", b.Rules)
		}
		if b.Diff == "" {
			t.Fatalf("empty bundle diff for %s", f)
		}
	}
}

func TestBuildEmptyDiff(t *testing.T) {
	repo, run := gitRepo(t)
	writeFile(t, repo, "a.go", "package a\n")
	run("add", ".")
	run("commit", "-qm", "init")

	p, err := Build(Options{Repo: repo, Mode: "worktree", HomeDir: t.TempDir(), RunID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Bundles) != 0 {
		t.Fatalf("expected 0 bundles, got %d", len(p.Bundles))
	}
}

func TestBuildNotARepo(t *testing.T) {
	if _, err := Build(Options{Repo: t.TempDir(), Mode: "worktree", RunID: "r1"}); err == nil {
		t.Fatal("expected error for non-repo")
	}
}
