//go:build windows

package sandbox

import "os/exec"

// On Windows the default CommandContext kill is used; the runner binary is
// the only consumer there and Playwright cleans up its own browser children.
func configureProcessGroup(cmd *exec.Cmd) {}
