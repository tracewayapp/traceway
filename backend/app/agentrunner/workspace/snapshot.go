package workspace

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Snapshot owns trusted Git metadata outside the directory mounted writable
// for tests. Nothing under the agent's .git directory is read or executed.
type Snapshot struct {
	Dir    string
	GitDir string
	root   string
	remote string
}

func NewSnapshot(ctx context.Context, mirror, remote, ref, source string) (*Snapshot, error) {
	root, err := os.MkdirTemp("", "traceway-publish-")
	if err != nil {
		return nil, err
	}
	s := &Snapshot{root: root, Dir: filepath.Join(root, "repo"), GitDir: filepath.Join(root, "git"), remote: remote}
	if _, err = git(ctx, "", nil, "clone", "--no-hardlinks", "--no-checkout", "--quiet", "--separate-git-dir", s.GitDir, "--branch", ref, "--single-branch", mirror, s.Dir); err != nil {
		s.Close()
		return nil, err
	}
	if _, err = s.git(ctx, nil, "reset", "--mixed", "HEAD"); err != nil {
		s.Close()
		return nil, err
	}
	if err = copyTree(source, s.Dir); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Snapshot) Close() { _ = os.RemoveAll(s.root) }

func copyTree(source, destination string) error {
	root, err := os.OpenRoot(source)
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." {
			return nil
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(path))
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0700)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := root.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			in, err := root.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, in)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			return closeErr
		default:
			return fmt.Errorf("unsupported repository file %s (%s)", path, info.Mode())
		}
	})
}

func (s *Snapshot) git(ctx context.Context, cred *Credential, args ...string) ([]byte, error) {
	base := []string{"--git-dir=" + s.GitDir, "--work-tree=" + s.Dir, "-c", "user.name=Traceway Agent", "-c", "user.email=agent@tracewayapp.com"}
	return git(ctx, s.root, cred, append(base, args...)...)
}

func (s *Snapshot) Changes(ctx context.Context) ([]Change, error) {
	out, err := s.git(ctx, nil, "status", "--porcelain=v1", "--no-renames", "-uall", "-z")
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, entry := range strings.Split(string(out), "\x00") {
		if len(entry) >= 4 {
			changes = append(changes, Change{Status: strings.TrimSpace(entry[:2]), Path: entry[3:]})
		}
	}
	return changes, nil
}

func (s *Snapshot) Diff(ctx context.Context) (string, error) {
	if _, err := s.git(ctx, nil, "add", "-A"); err != nil {
		return "", err
	}
	out, err := s.git(ctx, nil, "diff", "--cached", "--binary", "--no-color", "--no-ext-diff", "--no-textconv")
	return string(out), err
}

func (s *Snapshot) Commit(ctx context.Context, branch, message string, cred *Credential) error {
	if _, err := s.git(ctx, nil, "check-ref-format", "--branch", branch); err != nil {
		return err
	}
	if _, err := s.git(ctx, nil, "checkout", "--quiet", "-B", branch); err != nil {
		return err
	}
	if _, err := s.git(ctx, nil, "commit", "--quiet", "-m", message); err != nil {
		return err
	}
	_, err := s.git(ctx, cred, "push", "--quiet", s.remote, "HEAD:refs/heads/"+branch)
	return err
}
