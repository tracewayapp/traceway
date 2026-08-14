//go:build windows

package browserexec

import "os/exec"

// On Windows the default CommandContext kill is used; Playwright's own
// process management cleans up browser children well enough there, and the
// runner binary is the only consumer.
func configureProcessGroup(cmd *exec.Cmd) {}
