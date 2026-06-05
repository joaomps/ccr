package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func writeJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFindingsClean(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "b001.json", `{"bundle_id":"b001","findings":[{"file":"a.go","line":1,"title":"t"}]}`)
	bf, dropped, err := LoadFindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bf) != 1 || len(bf[0].Findings) != 1 {
		t.Fatalf("bf=%+v", bf)
	}
	if len(dropped) != 0 {
		t.Fatalf("dropped=%+v", dropped)
	}
}

func TestLoadFindingsProseWrapped(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "b002.json",
		"Here is my review:\n```json\n{\"bundle_id\":\"b002\",\"findings\":[{\"file\":\"a.go\",\"line\":2,\"title\":\"t\"}]}\n```\nDone.")
	bf, _, err := LoadFindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bf) != 1 || bf[0].BundleID != "b002" {
		t.Fatalf("recover failed: %+v", bf)
	}
}

func TestLoadFindingsBad(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, dir, "bad.json", "not json at all no braces")
	bf, dropped, err := LoadFindings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(bf) != 0 {
		t.Fatalf("bf=%+v", bf)
	}
	if len(dropped) != 1 {
		t.Fatalf("expected 1 dropped, got %+v", dropped)
	}
}
