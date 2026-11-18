package grade

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// workspaceGit runs a read-only git command in a workspace the agent has
// written to. The setup suite's container shell can write .git/config, so
// the graders use git only for commands that run no configured program (file
// listings and ignore checks, never diff or status, which run filter drivers)
// and switch off the index hooks those could reach: core.fsmonitor and
// core.hooksPath. The repository is pinned to the workspace's own .git, which
// must be a directory (a `gitdir:` file would point git elsewhere), no global
// or system config applies, and the command is killed after 30 seconds (a
// config can name a FIFO).
func workspaceGit(work string, stdin []byte, args ...string) ([]byte, error) {
	gitDir := filepath.Join(work, ".git")
	if fi, err := os.Lstat(gitDir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", gitDir)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.fsmonitor=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	cmd.Dir = work
	cmd.Env = []string{"HOME=" + work, "PATH=" + os.Getenv("PATH"), "GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + work,
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0"}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd.Output()
}

// gitIgnored reports which of paths a .gitignore (or info/exclude) excludes.
func gitIgnored(work string, paths []string) map[string]bool {
	out := map[string]bool{}
	if len(paths) == 0 {
		return out
	}
	raw, err := workspaceGit(work, []byte(strings.Join(paths, "\x00")+"\x00"), "check-ignore", "--no-index", "-z", "--stdin")
	var ee *exec.ExitError
	if err != nil && !(errors.As(err, &ee) && ee.ExitCode() == 1) {
		return out
	}
	for _, p := range strings.Split(string(raw), "\x00") {
		if p != "" {
			out[p] = true
		}
	}
	return out
}
