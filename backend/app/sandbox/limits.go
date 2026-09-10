package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/tracewayapp/traceway/backend/app/config"
)

var (
	limitsOnce      sync.Once
	systemdRunPath  string
	limitsSupported bool
)

// cgroupWrapper returns the systemd-run prefix that puts the process in a
// transient scope carrying the cpu, memory and pid limits, or nothing when
// the spec has no such limits or the host cannot enforce them. bwrap has no
// resource control of its own; a host without systemd (most containers)
// logs once that limits are unsupported and the process runs unlimited.
func cgroupWrapper(limits Limits) []string {
	if limits.CPU <= 0 && limits.MemoryMB <= 0 && limits.PIDs <= 0 {
		return nil
	}
	limitsOnce.Do(func() {
		if _, err := os.Stat("/run/systemd/system"); err != nil {
			config.Logf("sandbox: cpu/memory/pid limits are unsupported on this host (no systemd); processes run unlimited under bwrap")
			return
		}
		path, err := exec.LookPath("systemd-run")
		if err != nil {
			config.Logf("sandbox: cpu/memory/pid limits are unsupported on this host (systemd-run not on PATH); processes run unlimited under bwrap")
			return
		}
		systemdRunPath = path
		limitsSupported = true
	})
	if !limitsSupported {
		return nil
	}
	wrapper := []string{systemdRunPath, "--user", "--scope", "--quiet", "--collect"}
	if limits.CPU > 0 {
		wrapper = append(wrapper, "-p", fmt.Sprintf("CPUQuota=%d%%", int(limits.CPU*100)))
	}
	if limits.MemoryMB > 0 {
		wrapper = append(wrapper, "-p", fmt.Sprintf("MemoryMax=%dM", limits.MemoryMB))
	}
	if limits.PIDs > 0 {
		wrapper = append(wrapper, "-p", fmt.Sprintf("TasksMax=%d", limits.PIDs))
	}
	return wrapper
}
