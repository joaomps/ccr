// Package collect validates the untrusted findings produced by file-reviewer
// subagents, anchors each to an exact line, deduplicates, and emits the
// positioned result.
package collect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"ccr/internal/model"
)

// LoadFindings reads every *.json file in dir as a BundleFindings document,
// tolerating files that wrap the JSON object in surrounding prose. Files that
// cannot be recovered are reported as file-level drops.
func LoadFindings(dir string) ([]model.BundleFindings, []model.Dropped, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	var out []model.BundleFindings
	var dropped []model.Dropped
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			dropped = append(dropped, model.Dropped{Reason: "unreadable:" + e.Name()})
			continue
		}
		bf, ok := decodeBundleFindings(b)
		if !ok {
			dropped = append(dropped, model.Dropped{Reason: "schema_invalid:" + e.Name()})
			continue
		}
		out = append(out, bf)
	}
	return out, dropped, nil
}

func decodeBundleFindings(b []byte) (model.BundleFindings, bool) {
	var bf model.BundleFindings
	if err := json.Unmarshal(b, &bf); err == nil {
		return bf, true
	}
	if obj := extractFirstJSONObject(b); obj != nil {
		if err := json.Unmarshal(obj, &bf); err == nil {
			return bf, true
		}
	}
	return model.BundleFindings{}, false
}

// extractFirstJSONObject returns the first balanced {...} run, ignoring braces
// that appear inside JSON string literals.
func extractFirstJSONObject(b []byte) []byte {
	start, depth := -1, 0
	inStr, esc := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			depth--
			if depth == 0 && start >= 0 {
				return b[start : i+1]
			}
		}
	}
	return nil
}
