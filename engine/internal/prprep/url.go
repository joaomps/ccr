// Package prprep resolves a GitHub pull request URL into a local detached
// worktree at the PR head, using gh for metadata and git for materialization.
// It is the GitHub counterpart of the mrprep (GitLab) package.
package prprep

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const prMarker = "/pull/"

// ParsePRURL extracts the host, "owner/repo" project, and PR number from a
// GitHub pull request URL such as
// https://github.com/owner/repo/pull/42 (optionally /files, /commits, ?query).
func ParsePRURL(raw string) (host, project string, number int, err error) {
	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", 0, fmt.Errorf("invalid url: %w", perr)
	}
	if u.Host == "" {
		return "", "", 0, fmt.Errorf("url has no host: %q", raw)
	}
	idx := strings.Index(u.Path, prMarker)
	if idx < 0 {
		return "", "", 0, fmt.Errorf("not a pull request url: %q", raw)
	}
	project = strings.Trim(u.Path[:idx], "/")
	if strings.Count(project, "/") != 1 {
		return "", "", 0, fmt.Errorf("expected owner/repo in url: %q", raw)
	}
	rest := u.Path[idx+len(prMarker):]
	if s := strings.IndexByte(rest, '/'); s >= 0 {
		rest = rest[:s]
	}
	number, err = strconv.Atoi(rest)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number %q: %w", rest, err)
	}
	return u.Host, project, number, nil
}

// apiEndpoint builds the GitHub REST endpoint for a PR. owner and repo never
// contain slashes, so no escaping is needed.
func apiEndpoint(project string, number int) string {
	return fmt.Sprintf("repos/%s/pulls/%d", project, number)
}
