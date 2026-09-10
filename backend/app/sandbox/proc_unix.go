//go:build !windows

package sandbox

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup makes the child lead its own process group and kills
// the whole group on context cancellation, so nothing it spawned (a Chromium,
// a test suite's daemon) outlives a timed-out run.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
