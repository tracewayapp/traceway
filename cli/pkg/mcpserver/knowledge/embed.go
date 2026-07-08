// Package knowledge holds the canonical Traceway operator playbooks. The
// chunks are embedded into the MCP server (as instructions, resources, and
// prompt bodies) and assembled into the published Claude Code skill by
// tools/skillgen, so both surfaces share one source of truth. Edit the
// markdown here, then run 'just gen-skills'.
package knowledge

import "embed"

//go:embed *.md
var FS embed.FS

// MustRead returns the named chunk or panics: chunk names are compile-time
// constants, so a miss is a build defect, not a runtime condition.
func MustRead(name string) string {
	b, err := FS.ReadFile(name)
	if err != nil {
		panic("mcpserver knowledge: missing embedded chunk " + name + ": " + err.Error())
	}
	return string(b)
}
