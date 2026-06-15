package plan

import (
	"ccr/internal/model"
	"ccr/internal/rules"
)

// ignorePatterns are files never worth reviewing (vendored, generated, locks,
// minified). Matched with the same glob engine as rules.
var ignorePatterns = []string{
	"**/vendor/**",
	"**/node_modules/**",
	"**/dist/**",
	"**/build/**",
	"**/*.min.*",
	"**/*.lock",
	"**/go.sum",
	"go.sum",
	"**/*.pb.go",
	"**/package-lock.json",
	"**/yarn.lock",
	"**/pnpm-lock.yaml",
	"**/__pycache__/**",
	"**/.venv/**",
	"**/venv/**",
	"**/*.ipynb",
	"**/target/**",
	"**/dbt_packages/**",
	"**/*.parquet",
}

// Select partitions changed files into those worth reviewing and those skipped,
// recording the ignore pattern that excluded each skipped file.
func Select(files []string) (kept []string, skipped []model.Skipped) {
	for _, f := range files {
		matched := ""
		for _, pat := range ignorePatterns {
			if ok, _ := rules.Match(pat, f); ok {
				matched = pat
				break
			}
		}
		if matched != "" {
			skipped = append(skipped, model.Skipped{File: f, Reason: "ignored:" + matched})
		} else {
			kept = append(kept, f)
		}
	}
	return kept, skipped
}
