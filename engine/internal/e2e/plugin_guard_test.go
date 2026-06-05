package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoFile resolves a path relative to the repo root (three levels up from this
// package: internal/e2e -> internal -> engine -> repo).
func repoFile(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
}

func toolsLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "tools:") {
			return ln
		}
	}
	return ""
}

// TestSubagentsCanWriteFindings guards the seam the engine cannot test directly:
// both subagents are instructed to write their JSON output to a file, so their
// tools frontmatter MUST grant Write. Without it, collect reads an empty dir and
// every review reports zero findings.
func TestSubagentsCanWriteFindings(t *testing.T) {
	for _, name := range []string{"file-reviewer.md", "reflector.md"} {
		p := repoFile("plugin", "agents", name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		line := toolsLine(string(b))
		if line == "" {
			t.Fatalf("%s: no tools frontmatter line", name)
		}
		if !strings.Contains(line, "Write") {
			t.Fatalf("%s must grant the Write tool (it writes findings json), got: %q", name, line)
		}
		if !strings.Contains(line, "Read") {
			t.Fatalf("%s should grant Read, got: %q", name, line)
		}
	}
}

// TestReviewCommandWiring checks the orchestration command references the engine
// and both subagents, so the documented pipeline stays internally consistent.
func TestReviewCommandWiring(t *testing.T) {
	b, err := os.ReadFile(repoFile("plugin", "commands", "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"ccr-engine plan", "ccr-engine collect", "ccr-engine report", "file-reviewer", "reflector"} {
		if !strings.Contains(s, want) {
			t.Errorf("review.md missing reference to %q", want)
		}
	}
}
