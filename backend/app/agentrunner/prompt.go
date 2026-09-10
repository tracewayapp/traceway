package agentrunner

import (
	"fmt"
	"strings"
)

// harnessNotes is the part of the prompt the CI prepare action carried and
// the harness keeps: how the CLI is set up, what the agent must not do, and
// that the working tree is the pull request.
const harnessNotes = `Before anything else, read %s/SKILL.md and follow its Debug flow, treating the subject above as the issue reference; if the traceway skill is available to you as a skill, invoking it is the same thing. The skill explains how to get from the exception hash to the occurrence, and from there into traces, sessions and logs. The traceway CLI is installed and already authenticated against the right project with a read-only token: never run ` + "`traceway login`" + `, and never archive or otherwise change anything in Traceway.

You have NO access to git, gh or any network tool beyond the traceway CLI and your model provider; the harness commits, pushes and opens the pull request after you finish. Your job is only to investigate, edit files in the working tree if a fix is warranted, and end with the report.

If the investigation ties the root cause to code in this repository, fix the cause, not the symptom. Keep the diff minimal. Run the relevant tests for the packages you touch when a test command is available to you. If you cannot tie the root cause to code with confidence, do NOT guess a fix; leave the working tree untouched and report STATUS: analysis. If one piece of information from a human would let you decide, ask for it with STATUS: question and stop; the answer reaches you in the thread.

Do not add code comments explaining your change, why it is correct, or what the error was. The diff must read like the surrounding code and follow the repository's contribution rules (CLAUDE.md or its equivalent). ALL explanation belongs in the report, none of it in the code.

Never create helper scripts, notes, or any other scratch files inside the repository working tree: everything in the working tree becomes part of the pull request, and the harness refuses a run that leaves scratch files behind (%s). Use %s for anything temporary.`

// Prompt is the rendered context pack followed by the harness notes, which
// name paths inside the sandbox and so belong to the executor, not the
// control plane.
func Prompt(pack string, skillDir string, scratchDir string, scratchPatterns []string) string {
	var b strings.Builder
	b.WriteString(pack)
	b.WriteString("\n## Working rules\n")
	fmt.Fprintf(&b, harnessNotes, skillDir, strings.Join(scratchPatterns, " "), scratchDir)
	b.WriteString("\n")
	return b.String()
}

// verifyFailureMessage is what the agent hears when the repository's tests
// failed after its change; the tail is data, so it goes through the pack's
// thread on resume rather than into this text.
const verifyFailureMessage = "The repository's test command failed after your change. Its output is in the thread as data. Fix the cause or, if the failure is unrelated to your change and you can show why, say so in the report; leave the working tree in the state you want published."
