// skillgen assembles the published Claude Code skills from the canonical
// knowledge chunks embedded in pkg/mcpserver/knowledge, so the skills and the
// MCP server cannot drift. It writes the read-only skill to skills/traceway/,
// the product guide to skills/traceway-learn/, and the write skill to
// plugins/tw-write/skills/traceway-write/, a separate Claude Code plugin from
// the read-only tw one. Run via 'just gen-skills' from cli/.
//
// Note: skills-lock.json pins an installer-computed hash of the skill; after
// regenerating, reinstall the skill (or rerun the skill installer) to refresh
// the lock.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

//go:embed skill.tmpl.md
var skillTemplate string

//go:embed skill-write.tmpl.md
var writeSkillTemplate string

//go:embed skill-learn.tmpl.md
var learnSkillTemplate string

var learnTopics = []string{
	"issues.md",
	"endpoints-and-tasks.md",
	"traces-and-logs.md",
	"metrics-and-dashboards.md",
	"sessions-and-ai.md",
	"monitors.md",
	"alerts.md",
	"on-call.md",
	"organization.md",
}

func copiedHeader(file string) string {
	return "<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/" + file + " by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->\n"
}

// main takes an optional root to write into instead of the repository root,
// which is how gen-skills-check generates into a temp dir and diffs.
func main() {
	root := ".."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	readDir := filepath.Join(root, "skills", "traceway")
	write(filepath.Join(readDir, "SKILL.md"), render(skillTemplate))
	write(filepath.Join(readDir, "performance.md"), copiedHeader("performance.md")+knowledge.MustRead("performance.md"))

	learnDir := filepath.Join(root, "skills", "traceway-learn")
	write(filepath.Join(learnDir, "SKILL.md"), render(learnSkillTemplate))
	for _, topic := range learnTopics {
		write(filepath.Join(learnDir, topic), copiedHeader("learn/"+topic)+knowledge.MustRead("learn/"+topic))
	}

	writeDir := filepath.Join(root, "plugins", "tw-write", "skills", "traceway-write")
	write(filepath.Join(writeDir, "SKILL.md"), render(writeSkillTemplate))
}

func render(text string) string {
	tmpl := template.Must(template.New("skill").Funcs(template.FuncMap{
		"include": func(name string) (string, error) {
			b, err := knowledge.FS.ReadFile(name)
			if err != nil {
				return "", err
			}
			return strings.TrimRight(string(b), "\n"), nil
		},
	}).Parse(text))

	var sb strings.Builder
	if err := tmpl.Execute(&sb, nil); err != nil {
		fatal(err)
	}
	return sb.String()
}

func write(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Println("generated", path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "skillgen:", err)
	os.Exit(1)
}
