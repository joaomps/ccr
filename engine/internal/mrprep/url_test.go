package mrprep

import "testing"

func TestParseMRURL(t *testing.T) {
	cases := []struct {
		raw     string
		host    string
		project string
		iid     int
		ok      bool
	}{
		{"https://gitlab.com/group/project/-/merge_requests/42", "gitlab.com", "group/project", 42, true},
		{"https://gitlab.com/grp/sub/project/-/merge_requests/7", "gitlab.com", "grp/sub/project", 7, true},
		{"https://gitlab.example.com/team/app/-/merge_requests/100/diffs", "gitlab.example.com", "team/app", 100, true},
		{"https://gitlab.com/grp/proj/-/merge_requests/5?tab=overview", "gitlab.com", "grp/proj", 5, true},
		{"https://gitlab.com/grp/proj/-/issues/5", "", "", 0, false},
		{"not a url with spaces", "", "", 0, false},
	}
	for _, c := range cases {
		h, p, iid, err := ParseMRURL(c.raw)
		if c.ok {
			if err != nil || h != c.host || p != c.project || iid != c.iid {
				t.Errorf("ParseMRURL(%q)=(%q,%q,%d,%v) want (%q,%q,%d)", c.raw, h, p, iid, err, c.host, c.project, c.iid)
			}
		} else if err == nil {
			t.Errorf("ParseMRURL(%q) expected error", c.raw)
		}
	}
}

func TestAPIEndpoint(t *testing.T) {
	got := apiEndpoint("grp/sub/proj", 5)
	want := "projects/grp%2Fsub%2Fproj/merge_requests/5"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
