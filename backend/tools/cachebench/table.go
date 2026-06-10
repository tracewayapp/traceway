package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func renderTable(resultsDir string) (string, error) {
	files, err := filepath.Glob(filepath.Join(resultsDir, "*.json"))
	if err != nil {
		return "", err
	}
	var rows []runResult
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return "", err
		}
		var r runResult
		if err := json.Unmarshal(data, &r); err != nil {
			return "", fmt.Errorf("parsing %s: %w", f, err)
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no result files in %s", resultsDir)
	}

	cells := make(map[float64]map[int]map[string]*runResult)
	var labels []string
	for i := range rows {
		r := &rows[i]
		if cells[r.ColdRatio] == nil {
			cells[r.ColdRatio] = make(map[int]map[string]*runResult)
		}
		if cells[r.ColdRatio][r.Entries] == nil {
			cells[r.ColdRatio][r.Entries] = make(map[string]*runResult)
		}
		cells[r.ColdRatio][r.Entries][r.Label] = r
		if !slices.Contains(labels, r.Label) {
			labels = append(labels, r.Label)
		}
	}
	slices.Sort(labels)
	if i := slices.Index(labels, "memory"); i > 0 {
		labels = append([]string{"memory"}, slices.Delete(labels, i, i+1)...)
	}
	ratios := slices.Sorted(maps.Keys(cells))

	var sb strings.Builder
	fmt.Fprintf(&sb, "# Symbolicator cache benchmark: in-memory vs mmap disk cache\n\n")
	fmt.Fprintf(&sb, "Instance: %s. Hot set: %d bundles. Workload: %d concurrent resolvers, 3 frames per stack trace.\n\n",
		rows[0].Tier, rows[0].Hot, rows[0].Concurrency)
	sb.WriteString("Peak RSS is the benchmark process's maximum resident memory over the whole cell (sampled every 200ms), so it captures the worst-case RAM cost of each cache design under that load.\n\n")

	for _, ratio := range ratios {
		fmt.Fprintf(&sb, "## Cold traffic ratio %.0f%% (share of lookups hitting a uniformly random bundle outside the hot set)\n\n", ratio*100)
		entrySizes := slices.Sorted(maps.Keys(cells[ratio]))

		ratioLabels := make([]string, 0, len(labels))
		for _, l := range labels {
			for _, entries := range entrySizes {
				if cells[ratio][entries][l] != nil {
					ratioLabels = append(ratioLabels, l)
					break
				}
			}
		}

		sb.WriteString("| Bundles | Resolver corpus |")
		for _, l := range ratioLabels {
			fmt.Fprintf(&sb, " %s rps | %s p99 ms | %s peak RSS MB |", l, l, l)
		}
		sb.WriteString(" Winner |\n")
		sb.WriteString("|---|---|")
		for range ratioLabels {
			sb.WriteString("---|---|---|")
		}
		sb.WriteString("---|\n")

		crossover := -1
		broken := -1
		for _, entries := range entrySizes {
			row := cells[ratio][entries]
			corpusGB := 0.0
			for _, c := range row {
				if c.CorpusGB > 0 {
					corpusGB = c.CorpusGB
				}
			}
			fmt.Fprintf(&sb, "| %d | %.2f GB |", entries, corpusGB)

			bestLabel, bestRPS := "", -1.0
			for _, l := range ratioLabels {
				c := row[l]
				if c == nil {
					sb.WriteString(" - | - | - |")
					continue
				}
				if c.Status != "ok" {
					fmt.Fprintf(&sb, " **BREAK (%s)** | - | - |", strings.ToUpper(c.Status))
					if c.Status == "oom" && strings.HasPrefix(l, "memory") && broken == -1 {
						broken = entries
					}
					continue
				}
				fmt.Fprintf(&sb, " %.0f | %.2f | %.0f |", c.RPS, c.P99Us/1000, c.RSSPeakMB)
				if c.RPS > bestRPS {
					bestRPS, bestLabel = c.RPS, l
				}
			}
			fmt.Fprintf(&sb, " %s |\n", bestLabel)

			if crossover == -1 && bestLabel == "disk" {
				if m := row["memory"]; m != nil && m.Status == "ok" {
					crossover = entries
				}
			}
		}
		sb.WriteString("\n")
		if crossover != -1 {
			fmt.Fprintf(&sb, "Disk cache becomes cheaper than the in-memory cache at **%d bundles** for this cold ratio.\n", crossover)
		}
		if broken != -1 {
			fmt.Fprintf(&sb, "The in-memory cache breaks at **%d bundles**; beyond this point the file-based cache is the only option.\n", broken)
		}
		if crossover != -1 || broken != -1 {
			sb.WriteString("\n")
		}

		sb.WriteString("<details><summary>Cache internals per cell</summary>\n\n")
		sb.WriteString("| Bundles | Label | Hits | Misses | Builds | Disk hits | Store hits | Disk evictions | Unresolved frames | GC pause ms |\n")
		sb.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
		for _, entries := range entrySizes {
			for _, l := range ratioLabels {
				r := cells[ratio][entries][l]
				if r == nil || r.Status != "ok" {
					continue
				}
				fmt.Fprintf(&sb, "| %d | %s | %d | %d | %d | %d | %d | %d | %d | %.0f |\n",
					entries, l, r.CacheHits, r.CacheMisses, r.Builds, r.DiskHits, r.StoreHits, r.DiskEvict, r.UnresolvedFrm, r.GCPauseMs)
			}
		}
		sb.WriteString("\n</details>\n\n")
	}
	return sb.String(), nil
}
