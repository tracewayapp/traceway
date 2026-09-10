//go:build docker_live

package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These run against a real docker daemon: go test -tags docker_live ./app/sandbox/
// The image is alpine, small and with wget for the network checks.
const liveImage = "alpine:3.20"

// liveDir is a directory the docker daemon can bind-mount: t.TempDir on
// macOS lands outside Docker Desktop's shared paths, so DOCKER_LIVE_TMP
// overrides it.
func liveDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DOCKER_LIVE_TMP")
	if base == "" {
		return t.TempDir()
	}
	dir, err := os.MkdirTemp(base, "sandbox-live-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func liveDocker(t *testing.T) *Docker {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not on PATH")
	}
	d := &Docker{Image: liveImage, ScratchMB: 64}
	if err := d.Probe(context.Background(), Spec{Command: []string{"/bin/sh", "-c", "true"}, Workdir: "/tmp"}); err != nil {
		t.Skipf("docker probe failed: %v", err)
	}
	return d
}

func runLive(t *testing.T, d *Docker, spec Spec) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd, cleanup, err := d.Build(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestDockerLiveRootfsReadOnlyWorkspaceWritable(t *testing.T) {
	d := liveDocker(t)
	work := liveDir(t)
	out, err := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "touch /usr/marker 2>/dev/null && echo ROOTFS_WRITABLE; echo hello > " + work + "/out.txt && echo WORK_OK; touch /tmp/scratch && echo TMP_OK"},
		Workdir:  work,
		Writable: []Bind{{Path: work}},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
	})
	if err != nil {
		t.Fatalf("run: %v: %s", err, out)
	}
	if strings.Contains(out, "ROOTFS_WRITABLE") {
		t.Fatalf("root filesystem was writable: %s", out)
	}
	if !strings.Contains(out, "WORK_OK") || !strings.Contains(out, "TMP_OK") {
		t.Fatalf("workspace or scratch not writable: %s", out)
	}
	content, err := os.ReadFile(filepath.Join(work, "out.txt"))
	if err != nil || strings.TrimSpace(string(content)) != "hello" {
		t.Fatalf("workspace file on the host = %q, %v", content, err)
	}
	info, _ := os.Stat(filepath.Join(work, "out.txt"))
	if info == nil {
		t.Fatal("no file")
	}
}

func TestDockerLiveReadOnlyBindAndHiddenSiblings(t *testing.T) {
	d := liveDocker(t)
	root := liveDir(t)
	ro := filepath.Join(root, "ro")
	sibling := filepath.Join(root, "work", "other")
	mine := filepath.Join(root, "work", "mine")
	for _, dir := range []string{ro, sibling, mine} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(ro, "f"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(sibling, "secret"), []byte("x"), 0o644)
	out, err := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "cat " + ro + "/f && (touch " + ro + "/g 2>/dev/null && echo RO_WRITABLE || echo RO_OK); ls " + sibling + " 2>/dev/null && echo SIBLING_VISIBLE || echo SIBLING_HIDDEN"},
		Workdir:  mine,
		ReadOnly: []Bind{{Path: ro}},
		Writable: []Bind{{Path: mine}},
		Hidden:   []string{filepath.Join(root, "work")},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
	})
	if err != nil {
		t.Fatalf("run: %v: %s", err, out)
	}
	if !strings.Contains(out, "RO_OK") || !strings.Contains(out, "SIBLING_HIDDEN") {
		t.Fatalf("isolation: %s", out)
	}
}

func TestDockerLiveLimitsApplied(t *testing.T) {
	d := liveDocker(t)
	work := liveDir(t)
	out, err := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "cat /sys/fs/cgroup/pids.max; cat /sys/fs/cgroup/memory.max"},
		Workdir:  work,
		Writable: []Bind{{Path: work}},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
		Limits:   Limits{PIDs: 64, MemoryMB: 256, CPU: 1},
	})
	if err != nil {
		t.Fatalf("run: %v: %s", err, out)
	}
	if !strings.Contains(out, "64") || !strings.Contains(out, "268435456") {
		t.Fatalf("limits not applied: %s", out)
	}
}

func TestDockerLiveWallClock(t *testing.T) {
	d := liveDocker(t)
	work := liveDir(t)
	start := time.Now()
	_, err := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "sleep 30"},
		Workdir:  work,
		Writable: []Bind{{Path: work}},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
		Limits:   Limits{WallClock: 3 * time.Second},
	})
	if err == nil {
		t.Fatal("sleep 30 finished under a 3s wall clock")
	}
	if time.Since(start) > 20*time.Second {
		t.Fatalf("wall clock did not stop the container: %s", time.Since(start))
	}
}

func TestDockerLiveNetworkOff(t *testing.T) {
	d := liveDocker(t)
	work := liveDir(t)
	out, _ := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "wget -T 5 -q -O - http://example.com >/dev/null 2>&1 && echo REACHED || echo BLOCKED"},
		Workdir:  work,
		Writable: []Bind{{Path: work}},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
		Network:  NetworkPolicy{Mode: NetworkOff},
	})
	if !strings.Contains(out, "BLOCKED") {
		t.Fatalf("network off still reached the internet: %s", out)
	}
}

func TestDockerLiveEgressAllowlist(t *testing.T) {
	d := liveDocker(t)
	if !d.EgressEnforced() {
		t.Skip("egress allow-list needs Linux, root and iptables")
	}
	work := liveDir(t)
	out, _ := runLive(t, d, Spec{
		Command:  []string{"/bin/sh", "-c", "wget -T 5 -q -O - https://api.github.com >/dev/null 2>&1 && echo ALLOWED_OK || echo ALLOWED_BLOCKED; wget -T 5 -q -O - http://example.com >/dev/null 2>&1 && echo OTHER_REACHED || echo OTHER_BLOCKED"},
		Workdir:  work,
		Writable: []Bind{{Path: work}},
		Env:      map[string]string{"PATH": "/usr/bin:/bin"},
		Network:  NetworkPolicy{Mode: NetworkAllow, Allow: []HostPort{{Host: "api.github.com", Port: 443}}},
	})
	if !strings.Contains(out, "ALLOWED_OK") || !strings.Contains(out, "OTHER_BLOCKED") {
		t.Fatalf("allow-list: %s", out)
	}
}
