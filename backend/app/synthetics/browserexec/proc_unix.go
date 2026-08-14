//go:build !windows

package browserexec

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup makes the node process lead its own process group and
// kills the whole group on context cancellation, so orphaned Chromium
// processes never outlive a timed-out run.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
