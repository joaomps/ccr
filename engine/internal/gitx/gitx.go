// Package gitx wraps the git CLI for the diff acquisition each review mode needs.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func run(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}

// IsRepo reports whether repo is inside a git work tree.
func IsRepo(repo string) bool {
	out, err := run(repo, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// DiffWorktree returns tracked changes against HEAD (staged + unstaged).
// New untracked files are not included.
func DiffWorktree(repo string) (string, error) {
	return run(repo, "diff", "HEAD")
}

// DiffRange returns the diff of `to` relative to the merge-base with `from`
// (git's three-dot range), which is the right view for branch review.
func DiffRange(repo, from, to string) (string, error) {
	return run(repo, "diff", from+"..."+to)
}

// DiffCommit returns the diff a single commit introduced.
func DiffCommit(repo, sha string) (string, error) {
	return run(repo, "diff", sha+"^", sha)
}

// MergeBase returns the best common ancestor of two refs.
func MergeBase(repo, a, b string) (string, error) {
	out, err := run(repo, "merge-base", a, b)
	return strings.TrimSpace(out), err
}
