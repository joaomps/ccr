package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	repo := t.TempDir()
	git(t, repo, "init", "-q")
	return repo
}

func write(t *testing.T, repo, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsRepo(t *testing.T) {
	repo := initRepo(t)
	if !IsRepo(repo) {
		t.Fatal("expected IsRepo true")
	}
	if IsRepo(t.TempDir()) {
		t.Fatal("expected IsRepo false for non-repo")
	}
}

func TestDiffWorktree(t *testing.T) {
	repo := initRepo(t)
	write(t, repo, "a.go", "package a\n\nfunc A() int { return 1 }\n")
	git(t, repo, "add", "a.go")
	git(t, repo, "commit", "-qm", "init")
	write(t, repo, "a.go", "package a\n\nfunc A() int { return 2 }\n")
	d, err := DiffWorktree(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "return 2") {
		t.Fatalf("diff missing change: %s", d)
	}
}

func TestDiffRangeAndCommit(t *testing.T) {
	repo := initRepo(t)
	write(t, repo, "a.go", "package a\nfunc A() {}\n")
	git(t, repo, "add", "a.go")
	git(t, repo, "commit", "-qm", "c1")
	write(t, repo, "a.go", "package a\nfunc A() { println(1) }\n")
	git(t, repo, "add", "a.go")
	git(t, repo, "commit", "-qm", "c2")

	d, err := DiffRange(repo, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d, "println(1)") {
		t.Fatalf("range diff: %s", d)
	}
	dc, err := DiffCommit(repo, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dc, "println(1)") {
		t.Fatalf("commit diff: %s", dc)
	}
}
