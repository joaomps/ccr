package prprep

import (
	"encoding/json"
	"testing"
)

func TestParsePRURL(t *testing.T) {
	cases := []struct {
		raw     string
		host    string
		project string
		number  int
		ok      bool
	}{
		{"https://github.com/owner/repo/pull/42", "github.com", "owner/repo", 42, true},
		{"https://github.com/owner/repo/pull/7/files", "github.com", "owner/repo", 7, true},
		{"https://github.com/owner/repo/pull/5?w=1", "github.com", "owner/repo", 5, true},
		{"https://github.example.com/team/app/pull/100/commits", "github.example.com", "team/app", 100, true},
		{"https://github.com/owner/repo/issues/5", "", "", 0, false},
		{"https://github.com/owner/repo/pull/abc", "", "", 0, false},
		{"https://github.com/owner/sub/repo/pull/5", "", "", 0, false}, // owner/repo only
		{"not a url with spaces", "", "", 0, false},
	}
	for _, c := range cases {
		h, p, n, err := ParsePRURL(c.raw)
		if c.ok {
			if err != nil || h != c.host || p != c.project || n != c.number {
				t.Errorf("ParsePRURL(%q)=(%q,%q,%d,%v) want (%q,%q,%d)", c.raw, h, p, n, err, c.host, c.project, c.number)
			}
		} else if err == nil {
			t.Errorf("ParsePRURL(%q) expected error", c.raw)
		}
	}
}

func TestAPIEndpoint(t *testing.T) {
	got := apiEndpoint("owner/repo", 5)
	want := "repos/owner/repo/pulls/5"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPRAPIDecode(t *testing.T) {
	payload := `{"number":42,"base":{"ref":"main","sha":"base1"},"head":{"ref":"feat","sha":"head1"}}`
	var m prAPI
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatal(err)
	}
	if m.Number != 42 || m.Base.Ref != "main" || m.Head.SHA != "head1" {
		t.Fatalf("meta=%+v", m)
	}
}
