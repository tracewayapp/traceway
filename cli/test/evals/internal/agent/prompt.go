package agent

import (
	"fmt"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// PromptInput is what an arm contributes to the system prompt. Everything
// else is identical across models and arms.
type PromptInput struct {
	SubmitTool string
	// Skills lists installed skills (name: description), as agent harnesses
	// advertise them.
	Skills []tools.Skill
	// Preloaded is the SKILL.md body of a slash-invoked skill.
	Preloaded *tools.Skill
	// MCPInstructions are the connected server's instructions.
	MCPInstructions string
	// Environment describes the machine (CLI installed and logged in, or an
	// MCP server connected).
	Environment string
}

// SystemPrompt builds the neutral coding-agent system prompt. It carries no
// timestamps so OpenAI's automatic prefix cache keeps hitting.
func SystemPrompt(in PromptInput) string {
	var b strings.Builder
	b.WriteString(`You are a software engineering agent working for a developer. You work in their repository, which is the current directory, and you act only through the tools provided. You cannot see their screen.

Work like a careful senior engineer: gather evidence before you conclude, quantify what you claim, and do not change anything the user did not ask you to change. When you need a decision only the user can make, ask with ask_user.

Every tool result starts with a line like [call c07]. When you cite evidence, cite those ids.
`)
	fmt.Fprintf(&b, "\nWhen you are done, call %s exactly once with your final answer. Do not stop without calling it.\n", in.SubmitTool)
	if in.Environment != "" {
		b.WriteString("\n# Environment\n\n" + in.Environment + "\n")
	}
	if len(in.Skills) > 0 {
		b.WriteString("\n# Skills\n\nThese skills are installed. When a task matches a skill's description, load it with load_skill before you start and follow it. A skill's companion files are under .skills/<name>/ and can be read with read_file.\n\n")
		for _, s := range in.Skills {
			fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
		}
	}
	if in.MCPInstructions != "" {
		b.WriteString("\n# Connected MCP server: traceway\n\nIts tools are named mcp__traceway__<tool>. The server's instructions:\n\n")
		b.WriteString(in.MCPInstructions)
		b.WriteString("\n")
	}
	if in.Preloaded != nil {
		fmt.Fprintf(&b, "\n# The user invoked /%s\n\nThe %s skill is loaded; its instructions follow. Its companion files are under .skills/%s/.\n\n", in.Preloaded.Name, in.Preloaded.Name, in.Preloaded.Name)
		b.WriteString(in.Preloaded.Body)
		b.WriteString("\n")
	}
	return b.String()
}

// StripSlash turns "/traceway perf X ..." into "perf X ..." for arms that do
// not have the invoked skill, so every arm gets the same request.
func StripSlash(prompt string) string {
	p := strings.TrimLeft(prompt, " \n")
	if !strings.HasPrefix(p, "/") {
		return prompt
	}
	sp := strings.IndexAny(p, " \n")
	if sp < 0 {
		return ""
	}
	return strings.TrimLeft(p[sp:], " ")
}

// SlashSkill returns the skill name a prompt invokes ("/traceway ..."), or "".
func SlashSkill(prompt string) string {
	p := strings.TrimLeft(prompt, " \n")
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	end := strings.IndexAny(p, " \n")
	if end < 0 {
		end = len(p)
	}
	return p[1:end]
}
