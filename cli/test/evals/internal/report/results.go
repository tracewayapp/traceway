// Package report turns run results into results.json and summary.md,
// compares them with the committed baseline, and estimates cost from live
// OpenRouter prices.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

// Header identifies what was evaluated and who started it.
type Header struct {
	RunID          string            `json:"runId"`
	GitSHA         string            `json:"gitSha"`
	GitDirty       bool              `json:"gitDirty"`
	SkillHashes    map[string]string `json:"skillHashes"`
	KnowledgeHash  string            `json:"knowledgeHash"`
	BinariesHash   string            `json:"binariesHash"`
	CLIVersion     string            `json:"cliVersion"`
	BackendVersion string            `json:"backendVersion"`
	ModelsDigest   string            `json:"modelsDigest"`
	Models         map[string]string `json:"models"`
	StartedBy      string            `json:"startedBy"`
	Command        string            `json:"command"`
	Started        time.Time         `json:"started"`
	Finished       time.Time         `json:"finished"`
	MaxCostUSD     float64           `json:"maxCostUsd"`
	// ReplacedSpendUSD is what the runs a resume ran again had cost.
	ReplacedSpendUSD float64           `json:"replacedSpendUsd,omitempty"`
	Oracle           bool              `json:"oracle,omitempty"`
	CI               map[string]string `json:"ci,omitempty"`
	// Regraded is set when the offline graders were run again over the
	// stored run records (twevals regrade).
	Regraded string `json:"regraded,omitempty"`
}

// Results is results.json.
type Results struct {
	Header     Header           `json:"header"`
	Runs       []*runner.Result `json:"runs"`
	Aggregates Aggregates       `json:"aggregates"`
}

// NewHeader fills the provenance fields.
func NewHeader(repoRoot, runID string, models *catalog.Models, binHash string, cmdline []string) Header {
	h := Header{RunID: runID, SkillHashes: map[string]string{}, Models: map[string]string{}, Started: time.Now().UTC(), BinariesHash: binHash}
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}
	h.GitSHA = git("rev-parse", "HEAD")
	h.GitDirty = git("status", "--porcelain", "--", "backend", "cli", "skills", "plugins") != ""
	h.StartedBy = git("config", "user.email")
	if actor := os.Getenv("GITHUB_ACTOR"); actor != "" {
		h.StartedBy = actor + " (GitHub Actions)"
		h.CI = map[string]string{"event": os.Getenv("GITHUB_EVENT_NAME"), "workflow": os.Getenv("GITHUB_WORKFLOW_REF"),
			"runId": os.Getenv("GITHUB_RUN_ID"), "ref": os.Getenv("GITHUB_REF")}
	}
	for name, dir := range map[string]string{
		"traceway": "skills/traceway", "traceway-write": "plugins/tw-write/skills/traceway-write", "traceway-setup": "skills/traceway-setup",
	} {
		h.SkillHashes[name] = HashDir(filepath.Join(repoRoot, dir))
	}
	h.KnowledgeHash = HashDir(filepath.Join(repoRoot, "cli/pkg/mcpserver/knowledge"))
	h.CLIVersion = git("describe", "--tags", "--match", "cli/v*", "--abbrev=0")
	h.BackendVersion = git("describe", "--tags", "--match", "backend/v*", "--abbrev=0")
	if models != nil {
		h.ModelsDigest = models.Digest
		for k, m := range models.Agents {
			h.Models[k] = m.ID
		}
		if models.Judge != nil {
			h.Models["judge"] = models.Judge.ID
		}
	}
	h.Command = strings.Join(cmdline, " ")
	return h
}

// HashDir is the SHA-256 of a directory's files (paths and contents).
func HashDir(dir string) string {
	h := sha256.New()
	var files []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		raw, _ := os.ReadFile(f)
		h.Write([]byte(rel))
		h.Write(raw)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Save writes results.json and summary.md into dir.
func Save(dir string, r *Results) error {
	r.Aggregates = Aggregate(r.Runs)
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "results.json"), raw, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "summary.md"), []byte(Markdown(r)), 0o644)
}

// Load reads a results.json, or rebuilds one from the result.json files
// under a run directory (a shard that died before writing its summary).
func Load(dir string) (*Results, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err == nil {
		var r Results
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", dir, err)
		}
		return &r, nil
	}
	r := &Results{}
	err = filepath.WalkDir(filepath.Join(dir, "runs"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "result.json" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		var one runner.Result
		if json.Unmarshal(b, &one) == nil {
			r.Runs = append(r.Runs, &one)
		}
		return nil
	})
	if err != nil || len(r.Runs) == 0 {
		return nil, fmt.Errorf("%s: no results.json and no run results", dir)
	}
	return r, nil
}

// Merge combines shard results under one header. A run that appears in
// two parts (a merged directory passed with its own shards, or the same shard
// twice) is an error, unless replace is set: then the later part's run
// replaces the earlier one (a rerun of the same case, arm and model), and
// each replacement is listed in the returned notes, along with parts from
// different commits, models or binaries.
func Merge(parts []*Results, replace bool) (*Results, []string, error) {
	out := &Results{}
	total, replaced := 0.0, 0.0
	from := map[string]int{}
	var notes []string
	for i, p := range parts {
		if i == 0 {
			out.Header = p.Header
		} else {
			h := out.Header
			for _, d := range [][3]string{{"commit", h.GitSHA, p.Header.GitSHA}, {"models", h.ModelsDigest, p.Header.ModelsDigest}, {"binaries", h.BinariesHash, p.Header.BinariesHash}} {
				if d[1] != d[2] {
					notes = append(notes, fmt.Sprintf("part %d (%s) has a different %s than part 1 (%s): %q vs %q", i+1, p.Header.RunID, d[0], h.RunID, d[2], d[1]))
				}
			}
		}
		total += p.Header.MaxCostUSD
		replaced += p.Header.ReplacedSpendUSD
		if p.Header.Finished.After(out.Header.Finished) {
			out.Header.Finished = p.Header.Finished
		}
		for _, r := range p.Runs {
			if j, ok := from[r.Key()]; ok {
				if !replace {
					return nil, nil, fmt.Errorf("run %s is in part %d and part %d: pass each shard once, and not a merged directory together with its shards (or pass --replace for a rerun)", r.Key(), j+1, i+1)
				}
				notes = append(notes, fmt.Sprintf("run %s from part %d (%s) replaces the one in part %d", r.Key(), i+1, p.Header.RunID, j+1))
				for k, prev := range out.Runs {
					if prev.Key() == r.Key() {
						out.Runs = append(out.Runs[:k], out.Runs[k+1:]...)
						break
					}
				}
			}
			from[r.Key()] = i
			out.Runs = append(out.Runs, r)
		}
	}
	out.Header.MaxCostUSD = total
	out.Header.ReplacedSpendUSD = replaced
	sort.SliceStable(out.Runs, func(i, j int) bool { return out.Runs[i].Key() < out.Runs[j].Key() })
	return out, notes, nil
}
