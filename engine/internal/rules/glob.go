package rules

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Match reports whether path matches a glob pattern. Supported syntax:
//
//	*   matches any run of non-separator characters within a path segment
//	?   matches a single non-separator character
//	**  matches any number of path segments (including zero)
//	{a,b,c}  brace alternation (non-nested)
//
// Paths are normalized to forward slashes before matching.
func Match(pattern, path string) (bool, error) {
	path = filepath.ToSlash(path)
	for _, p := range expandBraces(pattern) {
		re, err := regexp.Compile(globToRegex(p))
		if err != nil {
			return false, err
		}
		if re.MatchString(path) {
			return true, nil
		}
	}
	return false, nil
}

// expandBraces turns "a/{x,y}.go" into ["a/x.go", "a/y.go"]. Multiple groups
// expand as a cartesian product. Unbalanced braces are treated literally.
func expandBraces(p string) []string {
	open := strings.IndexByte(p, '{')
	if open < 0 {
		return []string{p}
	}
	rel := strings.IndexByte(p[open:], '}')
	if rel < 0 {
		return []string{p}
	}
	closeIdx := open + rel
	prefix, suffix := p[:open], p[closeIdx+1:]
	var out []string
	for _, opt := range strings.Split(p[open+1:closeIdx], ",") {
		out = append(out, expandBraces(prefix+opt+suffix)...)
	}
	return out
}

// globToRegex converts a single (brace-free) glob to an anchored regexp.
func globToRegex(pat string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pat); {
		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				i += 2
				if i < len(pat) && pat[i] == '/' {
					i++
					// "**/" matches zero or more whole segments.
					b.WriteString("(?:[^/]*/)*")
				} else {
					// trailing or bare "**" matches anything.
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		case '/':
			b.WriteString("/")
			i++
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
			i++
		}
	}
	b.WriteString("$")
	return b.String()
}
