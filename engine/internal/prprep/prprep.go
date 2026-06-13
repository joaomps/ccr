package prprep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ccr/internal/gitx"
)

// Result is the output of Prep: where the PR head is checked out and the SHAs
// needed to diff it. Field names mirror mrprep.Result so the orchestrator
// consumes both identically.
type Result struct {
	Repo    string `json:"repo"`    // repo holding the worktree (local or clone)
	Workdir string `json:"workdir"` // detached worktree at PR head
	BaseSHA string `json:"base_sha"`
	HeadSHA string `json:"head_sha"`
	PR      PRInfo `json:"pr"`
}

// PRInfo identifies the resolved pull request (kept for future comment posting,
// even though v1 only prints to the terminal).
type PRInfo struct {
	URL     string `json:"url"`
	Number  int    `json:"number"`
	Project string `json:"project"`
	Host    string `json:"host"`
}

// prAPI is the subset of the GitHub PR API we consume.
type prAPI struct {
	Number int `json:"number"`
	Base   struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"base"`
	Head struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	} `json:"head"`
}

// Prep resolves rawURL and materializes a detached worktree at the PR head.
// repo is the candidate local checkout; if it is not the PR's project, the
// project is cloned to a temp dir instead. Fork PRs work because GitHub exposes
// the head commit on the base repo as the pull/<n>/head ref.
func Prep(rawURL, repo string) (Result, error) {
	if _, err := exec.LookPath("gh"); err != nil {
		return Result{}, fmt.Errorf("gh not found on PATH; install gh and run `gh auth login`")
	}
	host, project, number, err := ParsePRURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	raw, err := ghAPI(host, apiEndpoint(project, number))
	if err != nil {
		return Result{}, err
	}
	var meta prAPI
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Result{}, fmt.Errorf("decoding PR metadata: %w", err)
	}
	if meta.Head.SHA == "" {
		return Result{}, fmt.Errorf("could not determine PR head sha")
	}

	used := repo
	if !gitx.IsRepo(used) || !originMatches(used, project) {
		clone, err := os.MkdirTemp("", "ccr-clone-*")
		if err != nil {
			return Result{}, err
		}
		if err := ghClone(host, project, clone); err != nil {
			return Result{}, err
		}
		used = clone
	}

	refs := []string{fmt.Sprintf("pull/%d/head", number)}
	if meta.Base.Ref != "" {
		refs = append(refs, meta.Base.Ref)
	}
	if err := gitx.Fetch(used, refs...); err != nil {
		return Result{}, fmt.Errorf("fetching PR refs: %w", err)
	}

	head := meta.Head.SHA
	// GitHub's "Files changed" view is a three-dot diff, so anchor on the
	// merge-base of the base branch and the PR head, not the raw base tip.
	base := ""
	if meta.Base.Ref != "" {
		if mb, err := gitx.MergeBase(used, "origin/"+meta.Base.Ref, head); err == nil {
			base = mb
		}
	}
	if base == "" {
		base = meta.Base.SHA
	}
	if base == "" {
		base = head + "^"
	}

	work, err := os.MkdirTemp("", "ccr-wt-*")
	if err != nil {
		return Result{}, err
	}
	if err := gitx.WorktreeAdd(used, work, head); err != nil {
		return Result{}, fmt.Errorf("creating worktree: %w", err)
	}

	return Result{
		Repo:    used,
		Workdir: work,
		BaseSHA: base,
		HeadSHA: head,
		PR:      PRInfo{URL: rawURL, Number: number, Project: project, Host: host},
	}, nil
}

// Cleanup removes the worktree created by Prep.
func Cleanup(repo, workdir string) error {
	return gitx.WorktreeRemove(repo, workdir)
}

func ghAPI(host, endpoint string) ([]byte, error) {
	cmd := exec.Command("gh", "api", endpoint)
	cmd.Env = ghEnv(host)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gh api %s: %v: %s", endpoint, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func ghClone(host, project, dir string) error {
	cmd := exec.Command("gh", "repo", "clone", project, dir)
	cmd.Env = ghEnv(host)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh repo clone %s: %v: %s", project, err, strings.TrimSpace(errb.String()))
	}
	return nil
}

// ghEnv sets GH_HOST for GitHub Enterprise hosts so gh targets the right API.
// For github.com it is left unset (gh's default).
func ghEnv(host string) []string {
	env := os.Environ()
	if host != "" && host != "github.com" {
		env = append(env, "GH_HOST="+host)
	}
	return env
}

func originMatches(repo, project string) bool {
	u, err := gitx.RemoteURL(repo)
	if err != nil {
		return false
	}
	return strings.Contains(u, project)
}
