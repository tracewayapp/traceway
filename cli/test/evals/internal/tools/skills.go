package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// Skill is one installed skill: a directory with SKILL.md and companions.
type Skill struct {
	Name        string
	Description string
	Dir         string // absolute
	Body        string // SKILL.md without frontmatter
}

// LoadSkills reads every <dir>/<name>/SKILL.md.
func LoadSkills(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		name, desc, body := ParseFrontmatter(string(raw))
		if name == "" {
			name = e.Name()
		}
		out = append(out, Skill{Name: name, Description: desc, Dir: filepath.Join(dir, e.Name()), Body: body})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ParseFrontmatter splits a SKILL.md into name, description and body.
func ParseFrontmatter(s string) (name, desc, body string) {
	if !strings.HasPrefix(s, "---\n") {
		return "", "", s
	}
	end := strings.Index(s[4:], "\n---")
	if end < 0 {
		return "", "", s
	}
	front := s[4 : 4+end]
	body = strings.TrimLeft(s[4+end+4:], "\n")
	for _, line := range strings.Split(front, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		if len(v) >= 2 && (v[0] == '\'' && v[len(v)-1] == '\'' || v[0] == '"' && v[len(v)-1] == '"') {
			v = v[1 : len(v)-1]
			v = strings.ReplaceAll(v, "''", "'")
		}
		switch strings.TrimSpace(k) {
		case "name":
			name = v
		case "description":
			desc = v
		}
	}
	return name, desc, body
}

// LoadSkill returns a skill's SKILL.md, the way agent harnesses load a
// skill on demand. Companion files are read with read_file.
type LoadSkill struct {
	Skills []Skill
	// Rel maps a skill's dir to its workspace-relative path (.skills/<name>).
	Rel    func(abs string) string
	Loaded *[]string
}

func (t *LoadSkill) Spec() Spec {
	var names []string
	for _, s := range t.Skills {
		names = append(names, s.Name)
	}
	return Spec{Name: "load_skill", Description: "Load an installed skill's instructions (its SKILL.md). Installed skills: " + strings.Join(names, ", ") + ". Files the skill mentions live next to it and can be read with read_file.",
		Parameters: obj(map[string]any{"name": map[string]any{"type": "string", "enum": names, "description": "Skill name."}}, "name")}
}

func (t *LoadSkill) Call(_ context.Context, raw json.RawMessage) Result {
	var in struct {
		Name string `json:"name"`
	}
	if err := decodeArgs(raw, &in); err != nil {
		return errResult("%v", err)
	}
	for _, s := range t.Skills {
		if s.Name == in.Name {
			if t.Loaded != nil {
				*t.Loaded = append(*t.Loaded, s.Name)
			}
			out := fmt.Sprintf("Skill %s (files in %s/):\n\n%s", s.Name, t.Rel(s.Dir), s.Body)
			return Result{Output: out, OK: true, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpLoadSkill, Args: map[string]any{"name": s.Name}}, OK: true}}}
		}
	}
	return Result{Output: "error: no skill named " + in.Name, Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpLoadSkill, Args: map[string]any{"name": in.Name}}}}}
}
