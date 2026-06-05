// Package mrprep resolves a GitLab merge request URL into a local detached
// worktree at the MR head, using glab for metadata and git for materialization.
// This is the only engine package that performs network I/O.
package mrprep

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const mrMarker = "/-/merge_requests/"

// ParseMRURL extracts the host, project path, and MR iid from a GitLab merge
// request URL such as
// https://gitlab.com/group/sub/project/-/merge_requests/42
func ParseMRURL(raw string) (host, project string, iid int, err error) {
	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", 0, fmt.Errorf("invalid url: %w", perr)
	}
	if u.Host == "" {
		return "", "", 0, fmt.Errorf("url has no host: %q", raw)
	}
	idx := strings.Index(u.Path, mrMarker)
	if idx < 0 {
		return "", "", 0, fmt.Errorf("not a merge request url: %q", raw)
	}
	project = strings.Trim(u.Path[:idx], "/")
	if project == "" {
		return "", "", 0, fmt.Errorf("no project path in url: %q", raw)
	}
	rest := u.Path[idx+len(mrMarker):]
	if s := strings.IndexByte(rest, '/'); s >= 0 {
		rest = rest[:s]
	}
	iid, err = strconv.Atoi(rest)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid MR iid %q: %w", rest, err)
	}
	return u.Host, project, iid, nil
}

// apiEndpoint builds the GitLab REST endpoint for an MR, URL-encoding the
// project path (slashes become %2F).
func apiEndpoint(project string, iid int) string {
	return fmt.Sprintf("projects/%s/merge_requests/%d", url.QueryEscape(project), iid)
}
