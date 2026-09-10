// Package workspace prepares the repository an attempt works in: a bare
// mirror per repository that every attempt fetches through, a checkout per
// attempt, and the checks the harness runs on the result before anything
// leaves the machine (the scratch-file guard, the diff limit, the secret
// scan). Credentials reach git through the process environment of the
// harness only; the agent's sandbox never sees them.
package workspace

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/agent"
)

const gitTimeout = 10 * time.Minute

// Credential is what git presents to the code host for one operation.
type Credential = agent.GitCredential

// Mirrors is the bare-clone cache: one mirror per integration and
// repository under Root, refreshed with a pruning fetch before each use and
// file-locked so concurrent attempts share a fetch instead of racing one.
type Mirrors struct {
	Root string
}

// Path is where a repository's mirror lives; the integration id keeps two
// tenants with the same owner/name apart.
func (m *Mirrors) Path(integrationId int, owner, name string) string {
	return filepath.Join(m.Root, fmt.Sprint(integrationId), owner, name+".git")
}

// Update clones the mirror on first use and fetches it afterwards, returning
// its path. The credential travels as a git config header in the child's
// environment, never in the URL, so it appears in no process listing and
// is not written into the mirror's config.
func (m *Mirrors) Update(ctx context.Context, integrationId int, owner, name, cloneURL string, cred *Credential) (string, error) {
	path := m.Path(integrationId, owner, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	unlock, err := lockFile(path + ".lock")
	if err != nil {
		return "", err
	}
	defer unlock()

	if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
		if _, err := git(ctx, "", cred, "clone", "--mirror", "--quiet", cloneURL, path); err != nil {
			return "", fmt.Errorf("clone mirror: %w", err)
		}
		return path, nil
	}
	if _, err := git(ctx, path, cred, "fetch", "--prune", "--quiet", cloneURL, "+refs/heads/*:refs/heads/*"); err != nil {
		return "", fmt.Errorf("fetch mirror: %w", err)
	}
	return path, nil
}

// Checkout clones the mirror into dir at ref and points origin at the real
// remote (without a credential) so a later push goes to the code host.
func Checkout(ctx context.Context, mirrorPath, cloneURL, ref, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	if _, err := git(ctx, "", nil, "clone", "--no-hardlinks", "--quiet", "--branch", ref, "--single-branch", mirrorPath, dir); err != nil {
		return fmt.Errorf("checkout %s: %w", ref, err)
	}
	for _, args := range [][]string{
		{"remote", "set-url", "origin", cloneURL},
		{"config", "user.name", "Traceway Agent"},
		{"config", "user.email", "agent@tracewayapp.com"},
	} {
		if _, err := git(ctx, dir, nil, args...); err != nil {
			return err
		}
	}
	return nil
}

// HasBranch reports whether the mirror carries a branch, so a resumed
// attempt can pick its fix branch back up.
func HasBranch(ctx context.Context, mirrorPath, branch string) bool {
	_, err := git(ctx, mirrorPath, nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// Change is one path the agent added, modified or removed.
type Change struct {
	Status string
	Path   string
}

// Changes lists the working tree changes, untracked files expanded so a new
// directory shows every file inside it instead of collapsing to "dir/".
func Changes(ctx context.Context, dir string) ([]Change, error) {
	out, err := git(ctx, dir, nil, "status", "--porcelain=v1", "-uall", "-z")
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, entry := range strings.Split(string(out), "\x00") {
		if len(entry) < 4 {
			continue
		}
		changes = append(changes, Change{Status: strings.TrimSpace(entry[:2]), Path: entry[3:]})
	}
	return changes, nil
}

// Diff stages everything and returns the patch of the staged tree against
// HEAD; staging is what the commit needs anyway.
func Diff(ctx context.Context, dir string) (string, error) {
	if _, err := git(ctx, dir, nil, "add", "-A"); err != nil {
		return "", err
	}
	out, err := git(ctx, dir, nil, "diff", "--cached", "--binary", "--no-color")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// DefaultScratchPatterns are the basename globs the publish action refuses:
// helper scripts, notes and dumps an agent leaves behind would otherwise
// ship in the pull request.
var DefaultScratchPatterns = []string{".tw_*", "*.sh", "*.patch", "*.log", "*.tmp", "fix-report*", "*.md~", "notes.md"}

var ErrScratchFile = errors.New("the agent left a scratch file in the working tree")

// ScratchGuard refuses new files whose basename matches a scratch pattern.
func ScratchGuard(changes []Change, patterns []string) error {
	for _, change := range changes {
		if change.Status != "??" && change.Status != "A" {
			continue
		}
		base := filepath.Base(change.Path)
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, base); matched {
				return fmt.Errorf("%w: %s", ErrScratchFile, change.Path)
			}
		}
	}
	return nil
}

var ErrDiffTooLarge = errors.New("the diff is larger than the attempt allows")

// DiffLimit refuses a patch above the byte or file limit; a runaway rewrite
// is not a fix a reviewer can read.
func DiffLimit(patch string, changes []Change, maxBytes int, maxFiles int) error {
	if maxBytes > 0 && len(patch) > maxBytes {
		return fmt.Errorf("%w: %d bytes, limit %d", ErrDiffTooLarge, len(patch), maxBytes)
	}
	if maxFiles > 0 && len(changes) > maxFiles {
		return fmt.Errorf("%w: %d files, limit %d", ErrDiffTooLarge, len(changes), maxFiles)
	}
	return nil
}

// Commit records the staged tree on a new branch and pushes it with the
// credential, returning nothing the agent could read back.
func Commit(ctx context.Context, dir, branch, message string, cred *Credential) error {
	if _, err := git(ctx, dir, nil, "checkout", "--quiet", "-b", branch); err != nil {
		return err
	}
	if _, err := git(ctx, dir, nil, "commit", "--quiet", "-m", message); err != nil {
		return err
	}
	if _, err := git(ctx, dir, cred, "push", "--quiet", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push %s: %w", branch, err)
	}
	return nil
}

func git(ctx context.Context, dir string, cred *Credential, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	args = append([]string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-c", "http.followRedirects=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "HOME=/nonexistent", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "GCM_INTERACTIVE=never"}
	if cred != nil {
		auth := base64.StdEncoding.EncodeToString([]byte(cred.Username + ":" + cred.Password))
		cmd.Env = append(cmd.Env,
			"GIT_CONFIG_COUNT=1",
			"GIT_CONFIG_KEY_0=http.extraheader",
			"GIT_CONFIG_VALUE_0=AUTHORIZATION: basic "+auth,
		)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}
