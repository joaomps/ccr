// Package rules resolves which review rules apply to a file path using a
// four-layer, first-match-wins priority chain:
//
//	1 (highest) --rule flag path
//	2           <repo>/.ccr/rule.json
//	3           <home>/.ccr/rule.json
//	4 (lowest)  embedded system_rules.json
//
// Within a rule file, layer entries are evaluated in declaration order and the
// first whose glob matches wins (even if its rule list is empty, which marks
// the file "covered" and stops fall-through).
package rules

import (
	"encoding/json"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"ccr/internal/model"
)

//go:embed system_rules.json
var systemRulesJSON []byte

// LayerEntry maps a glob to the rules that apply to matching files.
type LayerEntry struct {
	Path  string       `json:"path"`
	Rules []model.Rule `json:"rules"`
}

// RuleFile is one layer's worth of rules (one JSON document).
type RuleFile struct {
	Layers []LayerEntry `json:"layers"`
}

// MatchFile returns the rules of the first entry whose Path matches, and true.
// Returns (nil, false) if no entry matches.
func (rf RuleFile) MatchFile(path string) ([]model.Rule, bool) {
	for _, e := range rf.Layers {
		ok, err := Match(e.Path, path)
		if err == nil && ok {
			return e.Rules, true
		}
	}
	return nil, false
}

type labeled struct {
	label string
	rf    RuleFile
}

// Resolver holds the loaded rule files in priority order (highest first).
type Resolver struct {
	files []labeled
}

// RulesFor returns the rules from the highest-priority file whose entry matches.
func (r Resolver) RulesFor(path string) []model.Rule {
	rules, _, _ := r.RulesForWithSource(path)
	return rules
}

// RulesForWithSource also reports which layer matched (for `rules check`).
func (r Resolver) RulesForWithSource(path string) ([]model.Rule, string, bool) {
	for _, lf := range r.files {
		if rules, ok := lf.rf.MatchFile(path); ok {
			return rules, lf.label, true
		}
	}
	return nil, "", false
}

func parseRuleFile(b []byte) (RuleFile, error) {
	var rf RuleFile
	if err := json.Unmarshal(b, &rf); err != nil {
		return rf, err
	}
	return rf, nil
}

// Resolve builds a Resolver from the four layers. An empty ruleFlagPath or
// homeDir skips that layer. A missing project/global file is skipped silently;
// a present-but-malformed file (any layer) is an error.
func Resolve(repoDir, ruleFlagPath, homeDir string) (Resolver, error) {
	var res Resolver

	if ruleFlagPath != "" {
		b, err := os.ReadFile(ruleFlagPath)
		if err != nil {
			return res, fmt.Errorf("reading --rule %s: %w", ruleFlagPath, err)
		}
		rf, err := parseRuleFile(b)
		if err != nil {
			return res, fmt.Errorf("parsing --rule %s: %w", ruleFlagPath, err)
		}
		res.files = append(res.files, labeled{"--rule:" + ruleFlagPath, rf})
	}

	if repoDir != "" {
		if rf, ok, err := loadOptional(filepath.Join(repoDir, ".ccr", "rule.json")); err != nil {
			return res, err
		} else if ok {
			res.files = append(res.files, labeled{"project", rf})
		}
	}

	if homeDir != "" {
		if rf, ok, err := loadOptional(filepath.Join(homeDir, ".ccr", "rule.json")); err != nil {
			return res, err
		} else if ok {
			res.files = append(res.files, labeled{"global", rf})
		}
	}

	rf, err := parseRuleFile(systemRulesJSON)
	if err != nil {
		return res, fmt.Errorf("parsing embedded system_rules.json: %w", err)
	}
	res.files = append(res.files, labeled{"embedded", rf})

	return res, nil
}

// loadOptional reads a rule file that may not exist.
// Missing => (_, false, nil). Present but malformed => error.
func loadOptional(path string) (RuleFile, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RuleFile{}, false, nil
		}
		return RuleFile{}, false, err
	}
	rf, err := parseRuleFile(b)
	if err != nil {
		return RuleFile{}, false, fmt.Errorf("parsing %s: %w", path, err)
	}
	return rf, true, nil
}
