package runner

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// history.yaml in a fixture describes the commits leading to its final tree,
// with ages relative to the run so `git log` lines up with the seeded
// deploys. Each commit is the final tree with an overlay applied: the files
// under its .history/ directory (only those that differ from the final
// tree) replace or add files, and `absent` lists files that did not exist
// yet. The final tree (the fixture minus .history and history.yaml) is
// committed last as HEAD.
type fixtureHistory struct {
	Commits []struct {
		Dir     string   `yaml:"dir"`
		Absent  []string `yaml:"absent"`
		Message string   `yaml:"message"`
		Age     string   `yaml:"age"`
		Author  string   `yaml:"author"`
	} `yaml:"commits"`
	Head struct {
		Message string `yaml:"message"`
		Age     string `yaml:"age"`
		Author  string `yaml:"author"`
	} `yaml:"head"`
}

// buildWorkspace copies a fixture repo into work and commits its history.
func buildWorkspace(evalsRoot, repo, work string, now time.Time) error {
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	git := func(env []string, args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append([]string{"HOME=" + work, "PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"}, env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := git(nil, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(work, ".git", "info", "exclude"), []byte(".skills/\n.twevals*\n"), 0o644)
	commit := func(msg, age, author string) error {
		if author == "" {
			author = "Dana Developer <dana@example.com>"
		}
		name, email, _ := strings.Cut(strings.TrimSuffix(author, ">"), " <")
		when := now.Add(-time.Hour)
		if age != "" {
			d, err := time.ParseDuration(age)
			if err != nil {
				return fmt.Errorf("history age %q: %w", age, err)
			}
			when = now.Add(-d)
		}
		date := when.UTC().Format(time.RFC3339)
		env := []string{"GIT_AUTHOR_NAME=" + name, "GIT_AUTHOR_EMAIL=" + email, "GIT_COMMITTER_NAME=" + name, "GIT_COMMITTER_EMAIL=" + email,
			"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
		if err := git(env, "add", "-A"); err != nil {
			return err
		}
		return git(env, "commit", "-q", "--allow-empty", "-m", msg)
	}
	if repo == "" {
		if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# service\n"), 0o644); err != nil {
			return err
		}
		return commit("Initial commit", "720h", "")
	}
	src := filepath.Join(evalsRoot, "testdata", "repos", repo)
	if _, err := os.Stat(src); err != nil {
		src = filepath.Join(evalsRoot, "testdata", "setup-templates", repo)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("fixture repo %s: not in testdata/repos or testdata/setup-templates", repo)
		}
	}
	var hist fixtureHistory
	if raw, err := os.ReadFile(filepath.Join(src, "history.yaml")); err == nil {
		if err := yaml.Unmarshal(raw, &hist); err != nil {
			return fmt.Errorf("%s/history.yaml: %w", repo, err)
		}
	}
	for _, c := range hist.Commits {
		if err := clearTree(work); err != nil {
			return err
		}
		if err := copyFixture(src, work); err != nil {
			return err
		}
		if c.Dir != "" {
			if err := copyFixture(filepath.Join(src, c.Dir), work); err != nil {
				return err
			}
		}
		for _, a := range c.Absent {
			if err := os.RemoveAll(filepath.Join(work, filepath.FromSlash(a))); err != nil {
				return err
			}
		}
		if err := commit(c.Message, c.Age, c.Author); err != nil {
			return err
		}
	}
	if err := clearTree(work); err != nil {
		return err
	}
	if err := copyFixture(src, work); err != nil {
		return err
	}
	msg := hist.Head.Message
	if msg == "" {
		msg = "Initial commit"
	}
	age := hist.Head.Age
	if age == "" && len(hist.Commits) == 0 {
		age = "720h"
	}
	return commit(msg, age, hist.Head.Author)
}

// clearTree removes everything in work except .git and .skills.
func clearTree(work string) error {
	entries, err := os.ReadDir(work)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == ".skills" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(work, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// copyFixture copies a fixture tree, skipping its history metadata. Files
// named *.tmpl are copied without the suffix (so Go tooling never compiles
// fixture go.mod files inside the evals module).
func copyFixture(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if rel == "." {
			return nil
		}
		if rel == "history.yaml" || rel == ".history" || strings.HasPrefix(rel, ".history"+string(filepath.Separator)) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, strings.TrimSuffix(rel, ".tmpl"))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		mode := os.FileMode(0o644)
		if info != nil && info.Mode()&0o111 != 0 {
			mode = 0o755
		}
		return os.WriteFile(target, raw, mode)
	})
}

// installSkills copies skill directories into work/.skills/<name>/.
func installSkills(repoRoot, work string, names []string) error {
	for _, n := range names {
		src, ok := skillSources[n]
		if !ok {
			return fmt.Errorf("unknown skill %s", n)
		}
		dst := filepath.Join(work, ".skills", n)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		if err := copyFixture(filepath.Join(repoRoot, src), dst); err != nil {
			return err
		}
		// The agent may read but never edit its skills.
		_ = filepath.WalkDir(dst, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				_ = os.Chmod(p, 0o444)
			}
			return nil
		})
	}
	return nil
}

// skillSources maps a skill name to its published directory in the repo.
var skillSources = map[string]string{
	"traceway":       "skills/traceway",
	"traceway-write": "plugins/tw-write/skills/traceway-write",
	"traceway-setup": "skills/traceway-setup",
}
