package rules

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "internal/svc/a.go", true},
		{"**/*.go", "a.go", true},
		{"**/*.go", "a.ts", false},
		{"**/*_test.go", "internal/x_test.go", true},
		{"**/*_test.go", "internal/x.go", false},
		{"**/*.{go,ts}", "a.go", true},
		{"**/*.{go,ts}", "pkg/b.ts", true},
		{"**/*.{go,ts}", "pkg/b.py", false},
		{"*.go", "a.go", true},
		{"*.go", "dir/a.go", false},
		{"**/vendor/**", "a/vendor/b.go", true},
		{"**/vendor/**", "vendor/b.go", true},
		{"**/vendor/**", "internal/a.go", false},
		{"**/*", "anything/here.x", true},
	}
	for _, c := range cases {
		got, err := Match(c.pattern, c.path)
		if err != nil {
			t.Fatalf("Match(%q,%q) error: %v", c.pattern, c.path, err)
		}
		if got != c.want {
			t.Errorf("Match(%q,%q)=%v want %v", c.pattern, c.path, got, c.want)
		}
	}
}
