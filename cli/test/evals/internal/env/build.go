package env

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// Binaries are the executables a run needs.
type Binaries struct {
	Backend  string // backend/cmd/traceway
	CLI      string // cli/cmd/traceway (read edition)
	CLIWrite string // cli/cmd/traceway -tags write
	Hash     string // tree hash they were built from
}

var (
	buildMu    sync.Mutex
	buildCache = map[string]*Binaries{}
)

// BuildBinaries builds the backend and both CLI editions from repoRoot, cached
// under cacheDir/bin/<tree-hash>/ so an unchanged tree never rebuilds.
func BuildBinaries(repoRoot, cacheDir string) (*Binaries, error) {
	buildMu.Lock()
	defer buildMu.Unlock()
	hash, err := TreeHash(repoRoot)
	if err != nil {
		return nil, err
	}
	if b, ok := buildCache[hash]; ok {
		return b, nil
	}
	dir := filepath.Join(cacheDir, "bin", hash)
	b := &Binaries{
		Backend:  filepath.Join(dir, "traceway-backend"),
		CLI:      filepath.Join(dir, "traceway"),
		CLIWrite: filepath.Join(dir, "traceway-write"),
		Hash:     hash,
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// buildMu only covers this process, and go test ./... runs one process
	// per package: without this lock each package that boots a backend
	// compiles it at the same time, several GB of compiler memory at once.
	unlock, err := lockFile(filepath.Join(cacheDir, "bin", ".build.lock"))
	if err != nil {
		return nil, err
	}
	defer unlock()
	steps := []struct {
		out, dir string
		args     []string
	}{
		{b.Backend, filepath.Join(repoRoot, "backend"), []string{"build", "-o", b.Backend, "./cmd/traceway"}},
		{b.CLI, filepath.Join(repoRoot, "cli"), []string{"build", "-o", b.CLI, "./cmd/traceway"}},
		{b.CLIWrite, filepath.Join(repoRoot, "cli"), []string{"build", "-tags", "write", "-o", b.CLIWrite, "./cmd/traceway"}},
	}
	for _, s := range steps {
		if _, err := os.Stat(s.out); err == nil {
			continue
		}
		// Build to a private path and rename, so a build killed halfway
		// never leaves a binary the Stat above would accept.
		tmp := fmt.Sprintf("%s.tmp-%d", s.out, os.Getpid())
		args := append([]string{}, s.args...)
		for i := range args {
			if args[i] == s.out {
				args[i] = tmp
			}
		}
		cmd := exec.Command("go", args...)
		cmd.Dir = s.dir
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("go %s in %s: %w\n%s", strings.Join(args, " "), s.dir, err, out)
		}
		if err := os.Rename(tmp, s.out); err != nil {
			return nil, err
		}
	}
	buildCache[hash] = b
	return b, nil
}

// lockFile takes an exclusive flock on path, waiting for any other holder.
// The kernel drops the lock when its holder exits, so a killed build never
// leaves it stuck.
func lockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() { f.Close() }, nil
}

// TreeHash fingerprints the backend and CLI sources: the committed trees,
// the uncommitted diff, and untracked files. The evals module itself is
// excluded so editing a case never rebuilds the binaries.
func TreeHash(repoRoot string) (string, error) {
	git := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
		return out, nil
	}
	h := sha256.New()
	paths := []string{"backend", "cli", ":(exclude)cli/test"}
	head, err := git("ls-tree", "-r", "HEAD", "--", "backend", "cli")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(head), "\n") {
		if !strings.Contains(line, "\tcli/test/") {
			h.Write([]byte(line))
		}
	}
	diff, err := git(append([]string{"diff", "HEAD", "--binary", "--"}, paths...)...)
	if err != nil {
		return "", err
	}
	h.Write(diff)
	untracked, err := git(append([]string{"ls-files", "-o", "--exclude-standard", "--"}, paths...)...)
	if err != nil {
		return "", err
	}
	for _, f := range strings.Split(strings.TrimSpace(string(untracked)), "\n") {
		if f == "" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(repoRoot, f))
		if err != nil {
			continue
		}
		h.Write([]byte(f))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))[:16], nil
}

// RepoRoot walks up from dir to the directory holding .git.
func RepoRoot(dir string) (string, error) {
	d, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("no git repository above %s", dir)
		}
		d = parent
	}
}
