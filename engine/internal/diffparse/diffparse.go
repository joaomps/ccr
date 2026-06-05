// Package diffparse parses unified git diff output into, per file, the set of
// added (reviewable) lines numbered by their absolute new-file line, each hunk's
// new-file start line (a positioning fallback anchor), and the file's raw diff
// section (handed to the reviewer subagent).
package diffparse

import (
	"strconv"
	"strings"

	"ccr/internal/model"
)

// FileDiff holds the reviewable lines and raw diff text for one changed file.
type FileDiff struct {
	NewPath     string
	Raw         string
	ReviewLines []model.ReviewLine // added lines, absolute new-file line numbers
	HunkStarts  []int              // new-file start line of each hunk
}

// Parse converts unified diff text into per-file FileDiffs. Deleted files
// (+++ /dev/null) and binary files are skipped.
func Parse(unified string) ([]FileDiff, error) {
	var files []FileDiff
	var cur *FileDiff
	var buf []string
	newLine := 0
	inHunk := false
	skipFile := false

	flush := func() {
		if cur != nil {
			cur.Raw = strings.Join(buf, "\n")
			if !skipFile && cur.NewPath != "" {
				files = append(files, *cur)
			}
		}
		cur = nil
	}

	for _, ln := range strings.Split(unified, "\n") {
		if strings.HasPrefix(ln, "diff --git ") {
			flush()
			cur = &FileDiff{}
			buf = []string{ln}
			inHunk, skipFile, newLine = false, false, 0
			continue
		}
		if cur != nil {
			buf = append(buf, ln)
		}
		switch {
		case strings.HasPrefix(ln, "Binary files "):
			skipFile = true
		case strings.HasPrefix(ln, "--- "):
			// old path; not needed
		case strings.HasPrefix(ln, "+++ "):
			p := strings.TrimSpace(strings.TrimPrefix(ln, "+++ "))
			if p == "/dev/null" {
				skipFile = true
			} else if cur != nil {
				cur.NewPath = stripPrefix(p)
			}
		case strings.HasPrefix(ln, "@@"):
			start, ok := parseHunkNewStart(ln)
			if !ok {
				continue
			}
			inHunk = true
			newLine = start
			if cur != nil && !skipFile {
				cur.HunkStarts = append(cur.HunkStarts, start)
			}
		default:
			if !inHunk || cur == nil || skipFile || ln == "" {
				continue
			}
			switch ln[0] {
			case '+':
				cur.ReviewLines = append(cur.ReviewLines, model.ReviewLine{Line: newLine, Code: ln[1:]})
				newLine++
			case ' ':
				newLine++
			case '-':
				// removed line: new-file counter unchanged
			case '\\':
				// "\ No newline at end of file"
			}
		}
	}
	flush()
	return files, nil
}

// stripPrefix removes git's a/ or b/ path prefix.
func stripPrefix(p string) string {
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		return p[2:]
	}
	return p
}

// parseHunkNewStart extracts c from a "@@ -a,b +c,d @@" header.
func parseHunkNewStart(ln string) (int, bool) {
	i := strings.IndexByte(ln, '+')
	if i < 0 {
		return 0, false
	}
	rest := ln[i+1:]
	end := strings.IndexAny(rest, " ,")
	if end >= 0 {
		rest = rest[:end]
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}
