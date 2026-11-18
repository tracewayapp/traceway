package grade

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// maxScanBytes caps a workspace file the graders read into memory.
const maxScanBytes = 16 << 20

// Snapshot fingerprints a workspace's files (outside .git, .skills/ and the
// harness's .twevals* entries) before the agent starts, so the diff after the
// run needs no git. Git would read the workspace's .git/config, whose filter
// drivers it runs on the host, and `git diff HEAD` misses a change the setup
// container committed.
func Snapshot(root string) (map[string]string, error) {
	out := map[string]string{}
	err := walkWorkspace(root, func(rel, abs string, d fs.DirEntry) error {
		h, err := fileDigest(abs, d)
		if err != nil {
			return err
		}
		out[rel] = h
		return nil
	})
	return out, err
}

func walkWorkspace(root string, fn func(rel, abs string, d fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".twevals") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || rel == ".skills" {
				return filepath.SkipDir
			}
			return nil
		}
		return fn(rel, p, d)
	})
}

// fileDigest identifies a file's content without following a symlink or
// opening anything but a regular file.
func fileDigest(abs string, d fs.DirEntry) (string, error) {
	switch t := d.Type(); {
	case t&fs.ModeSymlink != 0:
		target, err := os.Readlink(abs)
		return "symlink:" + target, err
	case !t.IsRegular():
		return "special:" + t.String(), nil
	}
	f, err := os.OpenFile(abs, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// changedFiles lists the files added, modified or removed since the snapshot.
// Added files that a .gitignore excludes (build output) are left out.
func changedFiles(root string, pristine map[string]string) ([]string, error) {
	if pristine == nil {
		return nil, fmt.Errorf("no snapshot of the workspace before the run")
	}
	now, err := Snapshot(root)
	if err != nil {
		return nil, err
	}
	var out, added []string
	for rel, h := range now {
		if before, ok := pristine[rel]; !ok {
			added = append(added, rel)
		} else if before != h {
			out = append(out, rel)
		}
	}
	for rel := range pristine {
		if _, ok := now[rel]; !ok {
			out = append(out, rel)
		}
	}
	ignored := gitIgnored(root, added)
	for _, rel := range added {
		if !ignored[rel] {
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	return out, nil
}

// readRegular reads a workspace file for a grader: only a regular file (no
// symlink, FIFO or device the setup container may have planted) of at most
// maxScanBytes.
func readRegular(abs string) ([]byte, bool) {
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxScanBytes {
		return nil, false
	}
	f, err := os.OpenFile(abs, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxScanBytes))
	return raw, err == nil
}
