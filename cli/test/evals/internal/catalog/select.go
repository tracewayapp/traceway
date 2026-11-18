package catalog

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Preset selects cases and arms (presets.yaml).
type Preset struct {
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	// Arms is "primary" (each suite's primary arm) or "all" (every arm a
	// case lists).
	Arms string `yaml:"arms"`
}

// Presets is presets.yaml.
type Presets struct {
	Presets     map[string]Preset   `yaml:"presets"`
	PrimaryArms map[string][]string `yaml:"primary_arms"`
}

// LoadPresets reads presets.yaml under root.
func LoadPresets(root string) (*Presets, error) {
	raw, err := os.ReadFile(filepath.Join(root, "presets.yaml"))
	if err != nil {
		return nil, err
	}
	var p Presets
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("presets.yaml: %w", err)
	}
	for name, pr := range p.Presets {
		if pr.Arms != "primary" && pr.Arms != "all" {
			return nil, fmt.Errorf("presets.yaml: %s: arms must be primary or all", name)
		}
	}
	for _, s := range Suites {
		if len(p.PrimaryArms[s]) == 0 {
			return nil, fmt.Errorf("presets.yaml: primary_arms.%s is missing", s)
		}
	}
	return &p, nil
}

// Selection is the user's case/arm filter. Cases (when set) overrides Preset
// for which cases run; Arms (when set) overrides the preset's arm policy.
type Selection struct {
	Preset string
	Cases  string // comma-separated ids or globs
	Suites string // comma-separated
	Tags   string // comma-separated, all must match
	Arms   string // comma-separated
	Shard  string // one suite
}

// Unit is one (case, arm) pair; the runner multiplies it by models and runs.
type Unit struct {
	Case *Case
	Arm  string
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Select applies a selection to the loaded cases.
func Select(cases []*Case, presets *Presets, sel Selection) ([]Unit, error) {
	var preset *Preset
	if sel.Preset != "" {
		p, ok := presets.Presets[sel.Preset]
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", sel.Preset)
		}
		preset = &p
	}
	globs := splitList(sel.Cases)
	suites := splitList(sel.Suites)
	tags := splitList(sel.Tags)
	arms := splitList(sel.Arms)
	for _, a := range arms {
		if !slices.Contains(AllArms, a) {
			return nil, fmt.Errorf("unknown arm %q", a)
		}
	}
	for _, s := range suites {
		if !slices.Contains(Suites, s) {
			return nil, fmt.Errorf("unknown suite %q", s)
		}
	}
	if sel.Shard != "" && !slices.Contains(Suites, sel.Shard) {
		return nil, fmt.Errorf("unknown shard %q (shards are suites)", sel.Shard)
	}

	var out []Unit
	matchedGlob := map[string]bool{}
	for _, c := range cases {
		if len(globs) > 0 {
			hit := false
			for _, g := range globs {
				if ok, _ := path.Match(g, c.ID); ok {
					hit, matchedGlob[g] = true, true
				}
			}
			if !hit {
				continue
			}
		} else if preset != nil {
			if !hasAll(c, preset.Tags) {
				continue
			}
		}
		if len(suites) > 0 && !slices.Contains(suites, c.Suite) {
			continue
		}
		if sel.Shard != "" && c.Suite != sel.Shard {
			continue
		}
		if !hasAll(c, tags) {
			continue
		}
		var caseArms []string
		switch {
		case len(arms) > 0:
			for _, a := range arms {
				if slices.Contains(c.Arms, a) {
					caseArms = append(caseArms, a)
				}
			}
		case preset != nil && preset.Arms == "all":
			caseArms = c.Arms
		default:
			for _, a := range presets.PrimaryArms[c.Suite] {
				if slices.Contains(c.Arms, a) {
					caseArms = append(caseArms, a)
				}
			}
			if len(caseArms) == 0 {
				caseArms = c.Arms[:1]
			}
		}
		for _, a := range caseArms {
			out = append(out, Unit{Case: c, Arm: a})
		}
	}
	for _, g := range globs {
		if !matchedGlob[g] {
			return nil, fmt.Errorf("case %q matches nothing", g)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the selection is empty")
	}
	return out, nil
}

func hasAll(c *Case, tags []string) bool {
	for _, t := range tags {
		if !c.HasTag(t) {
			return false
		}
	}
	return true
}
