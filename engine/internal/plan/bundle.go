package plan

import (
	"path"
	"sort"
	"strings"
)

// Bundle groups related files into review units. v1 heuristic, intentionally
// conservative: within the same directory, a Go file and its _test.go sibling
// are bundled, and resource files differing only by a two-letter locale tag
// (message_en.properties, message_zh.properties) are bundled. Everything else
// is its own bundle. Output order and bundle contents are deterministic.
func Bundle(files []string) [][]string {
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)

	used := make(map[string]bool, len(sorted))
	var bundles [][]string
	for _, f := range sorted {
		if used[f] {
			continue
		}
		group := []string{f}
		used[f] = true
		for _, g := range sorted {
			if used[g] {
				continue
			}
			if related(f, g) {
				group = append(group, g)
				used[g] = true
			}
		}
		bundles = append(bundles, group)
	}
	return bundles
}

func related(a, b string) bool {
	if path.Dir(a) != path.Dir(b) {
		return false
	}
	return testSibling(a, b) || testSibling(b, a) || localeSibling(a, b)
}

// testSibling reports whether b is a's Go test file (foo.go <-> foo_test.go).
func testSibling(a, b string) bool {
	if path.Ext(a) != ".go" || !strings.HasSuffix(b, "_test.go") {
		return false
	}
	return strings.TrimSuffix(path.Base(a), ".go") == strings.TrimSuffix(path.Base(b), "_test.go")
}

var localeExts = map[string]bool{
	".properties": true, ".json": true, ".yaml": true, ".yml": true,
	".po": true, ".resx": true, ".strings": true, ".arb": true, ".xml": true,
}

// localeSibling reports whether a and b are the same resource file in two
// two-letter locales (same directory, same extension).
func localeSibling(a, b string) bool {
	ext := path.Ext(a)
	if ext != path.Ext(b) || !localeExts[ext] {
		return false
	}
	la, oka := localeStem(path.Base(a), ext)
	lb, okb := localeStem(path.Base(b), ext)
	return oka && okb && la == lb
}

// localeStem("message_en.properties", ".properties") -> ("message", true).
func localeStem(base, ext string) (string, bool) {
	name := strings.TrimSuffix(base, ext)
	i := strings.LastIndexByte(name, '_')
	if i <= 0 {
		return "", false
	}
	tag := name[i+1:]
	if len(tag) != 2 {
		return "", false
	}
	for _, c := range tag {
		if c < 'a' || c > 'z' {
			return "", false
		}
	}
	return name[:i], true
}
