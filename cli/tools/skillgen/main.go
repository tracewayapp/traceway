// skillgen assembles the published Claude Code skill (skills/traceway/) from
// the canonical knowledge chunks embedded in pkg/mcpserver/knowledge, so the
// skill and the MCP server cannot drift. Run via 'just gen-skills' from cli/.
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

const perfHeader = "<!-- GENERATED FILE: copied from cli/pkg/mcpserver/knowledge/performance.md by cli/tools/skillgen. Edit there and run just gen-skills in cli/. -->\n"

func main() {
	outDir := "../skills/traceway"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	tmpl := template.Must(template.New("skill").Funcs(template.FuncMap{
		"include": func(name string) (string, error) {
			b, err := knowledge.FS.ReadFile(name)
			if err != nil {
				return "", err
			}
			return strings.TrimRight(string(b), "\n"), nil
		},
	}).Parse(skillTemplate))

	var sb strings.Builder
	if err := tmpl.Execute(&sb, nil); err != nil {
		fatal(err)
	}
	write(filepath.Join(outDir, "SKILL.md"), sb.String())
	write(filepath.Join(outDir, "performance.md"), perfHeader+knowledge.MustRead("performance.md"))
}

func write(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatal(err)
	}
	fmt.Println("generated", path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "skillgen:", err)
	os.Exit(1)
}
