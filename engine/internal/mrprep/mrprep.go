package mrprep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"ccr/internal/gitx"
	"ccr/internal/model"
)

// Result is the output of Prep: where the MR head is checked out and the refs
// needed to diff and (later) post comments.
type Result struct {
	Repo     string         `json:"repo"`    // the repo holding the worktree (local or clone)
	Workdir  string         `json:"workdir"` // detached worktree at MR head
	BaseSHA  string         `json:"base_sha"`
	HeadSHA  string         `json:"head_sha"`
	DiffRefs model.DiffRefs `json:"diff_refs"`
	MR       model.MRInfo   `json:"mr"`
}

// mrAPI is the subset of the GitLab MR API we consume.
type mrAPI struct {
	IID          int    `json:"iid"`
	TargetBranch string `json:"target_branch"`
	SourceBranch string `json:"source_branch"`
	SHA          string `json:"sha"`
	DiffRefs     struct {
		BaseSHA  string `json:"base_sha"`
		HeadSHA  string `json:"head_sha"`
		StartSHA string `json:"start_sha"`
	} `json:"diff_refs"`
}

// Prep resolves rawURL and materializes a detached worktree at the MR head.
// repo is the candidate local checkout; if it is not the MR's project, the
// project is cloned to a temp dir instead.
func Prep(rawURL, repo string) (Result, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return Result{}, fmt.Errorf("glab not found on PATH; install glab and run `glab auth login`")
	}
	host, project, iid, err := ParseMRURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	raw, err := glabAPI(host, apiEndpoint(project, iid))
	if err != nil {
		return Result{}, err
	}
	var meta mrAPI
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Result{}, fmt.Errorf("decoding MR metadata: %w", err)
	}

	head := meta.DiffRefs.HeadSHA
	if head == "" {
		head = meta.SHA
	}
	if head == "" {
		return Result{}, fmt.Errorf("could not determine MR head sha")
	}

	used := repo
	if !gitx.IsRepo(used) || !originMatches(used, project) {
		clone, err := os.MkdirTemp("", "ccr-clone-*")
		if err != nil {
			return Result{}, err
		}
		if err := glabClone(host, project, clone); err != nil {
			return Result{}, err
		}
		used = clone
	}

	refs := []string{fmt.Sprintf("merge-requests/%d/head", iid)}
	if meta.TargetBranch != "" {
		refs = append(refs, meta.TargetBranch)
	}
	if err := gitx.Fetch(used, refs...); err != nil {
		return Result{}, fmt.Errorf("fetching MR refs: %w", err)
	}

	base := meta.DiffRefs.BaseSHA
	if base == "" && meta.TargetBranch != "" {
		if mb, err := gitx.MergeBase(used, "origin/"+meta.TargetBranch, head); err == nil {
			base = mb
		}
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

	refsModel := model.DiffRefs{
		BaseSHA:  meta.DiffRefs.BaseSHA,
		HeadSHA:  meta.DiffRefs.HeadSHA,
		StartSHA: meta.DiffRefs.StartSHA,
	}
	return Result{
		Repo:     used,
		Workdir:  work,
		BaseSHA:  base,
		HeadSHA:  head,
		DiffRefs: refsModel,
		MR: model.MRInfo{
			URL: rawURL, IID: iid, Project: project, Host: host, DiffRefs: refsModel,
		},
	}, nil
}

func Cleanup(repo, workdir string) error {
	return gitx.WorktreeRemove(repo, workdir)
}

func glabAPI(host, endpoint string) ([]byte, error) {
	cmd := exec.Command("glab", "api", endpoint)
	cmd.Env = append(os.Environ(), "GITLAB_HOST="+host)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("glab api %s: %v: %s", endpoint, err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

func glabClone(host, project, dir string) error {
	cmd := exec.Command("glab", "repo", "clone", project, dir)
	cmd.Env = append(os.Environ(), "GITLAB_HOST="+host)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("glab repo clone %s: %v: %s", project, err, strings.TrimSpace(errb.String()))
	}
	return nil
}

func originMatches(repo, project string) bool {
	u, err := gitx.RemoteURL(repo)
	if err != nil {
		return false
	}
	return strings.Contains(u, project)
}
