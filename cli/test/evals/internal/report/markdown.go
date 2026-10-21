package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

// Markdown renders summary.md.
func Markdown(r *Results) string {
	a := r.Aggregates
	var b strings.Builder
	h := r.Header
	title := "Traceway evals"
	if h.Oracle {
		title += " (oracle, fake model, $0)"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "- **Run:** `%s`, started by %s\n", h.RunID, orDash(h.StartedBy))
	sha := h.GitSHA
	if len(sha) > 12 {
		sha = sha[:12]
	}
	dirty := ""
	if h.GitDirty {
		dirty = " (uncommitted changes)"
	}
	fmt.Fprintf(&b, "- **Commit:** `%s`%s, skill `traceway` %s, knowledge %s\n", orDash(sha), dirty, short(h.SkillHashes["traceway"]), short(h.KnowledgeHash))
	var models []string
	for k, v := range h.Models {
		models = append(models, fmt.Sprintf("%s=`%s`", k, v))
	}
	sort.Strings(models)
	fmt.Fprintf(&b, "- **Models:** %s\n", strings.Join(models, ", "))
	// An oracle run costs nothing, so its summary leaves money out.
	paid := !h.Oracle
	if paid {
		fmt.Fprintf(&b, "- **Spend:** $%.2f (judge $%.2f) of a $%.2f cap\n", a.SpendUSD+h.ReplacedSpendUSD, a.JudgeUSD, h.MaxCostUSD)
		if h.ReplacedSpendUSD > 0 {
			fmt.Fprintf(&b, "  (including $%.2f spent on runs that a resume ran again)\n", h.ReplacedSpendUSD)
		}
	}
	var st []string
	for k, v := range a.Statuses {
		st = append(st, fmt.Sprintf("%s %d", k, v))
	}
	sort.Strings(st)
	fmt.Fprintf(&b, "- **Runs:** %d (%s)\n\n", len(r.Runs), strings.Join(st, ", "))

	b.WriteString("## Pass rate by suite\n\n| Suite | Model | Arm | Pass | Rate |\n|---|---|---|---|---|\n")
	keys := sortedKeys(a.Suites)
	for _, k := range keys {
		s, m, arm := split3(k)
		v := a.Suites[k]
		fmt.Fprintf(&b, "| %s | %s | %s | %d/%d | %.0f%% |\n", s, m, arm, v.Pass, v.Total, v.Rate*100)
	}
	if len(a.SkillUplift) > 0 {
		b.WriteString("\n## Skill uplift (cli+skill minus cli-noskill)\n\n| Suite | Model | With skill | Without | Uplift |\n|---|---|---|---|---|\n")
		for _, d := range a.SkillUplift {
			fmt.Fprintf(&b, "| %s | %s | %.0f%% | %.0f%% | %+.0fpp |\n", d.Suite, d.Model, d.RateA*100, d.RateB*100, d.Delta)
		}
	}
	if len(a.CLIvsMCP) > 0 {
		b.WriteString("\n## CLI vs MCP\n\n| Suite | Model | CLI arm | MCP arm | CLI | MCP | Delta |\n|---|---|---|---|---|---|---|\n")
		for _, d := range a.CLIvsMCP {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %.0f%% | %.0f%% | %+.0fpp |\n", d.Suite, d.Model, d.A, d.B, d.RateA*100, d.RateB*100, d.Delta)
		}
	}
	if len(a.Models) > 0 {
		b.WriteString("\n## Model vs model (same cases)\n\n| Suite | A | B | A rate | B rate | Delta |\n|---|---|---|---|---|---|\n")
		for _, d := range a.Models {
			fmt.Fprintf(&b, "| %s | %s | %s | %.0f%% | %.0f%% | %+.0fpp |\n", d.Suite, d.A, d.B, d.RateA*100, d.RateB*100, d.Delta)
		}
	}
	if len(a.Efficiency) > 0 {
		b.WriteString("\n## Efficiency (reported, not part of the pass rate)\n\n| Model | Arm | Runs | Avg tool calls | Avg failed calls | Over a call limit |\n|---|---|---|---|---|---|\n")
		for _, k := range sortedKeys(a.Efficiency) {
			m, arm, _ := strings.Cut(k, "|")
			e := a.Efficiency[k]
			fmt.Fprintf(&b, "| %s | %s | %d | %.1f | %.1f | %d |\n", m, arm, e.Runs, e.AvgToolCalls, e.AvgFailedCalls, e.OverLimit)
		}
	}
	if paid {
		b.WriteString("\n## Cost (averages over passing runs)\n\n| Model | Passing runs | Avg cost | Avg tool calls | Avg input tokens | Avg output tokens | Cache hit rate | Total spend |\n|---|---|---|---|---|---|---|---|\n")
		for _, m := range sortedKeys(a.Cost) {
			c := a.Cost[m]
			fmt.Fprintf(&b, "| %s | %d | $%.3f | %.1f | %.0f | %.0f | %.0f%% | $%.2f |\n", m, c.PassingRuns, c.AvgCostUSD, c.AvgToolCalls, c.AvgInputTokens, c.AvgOutputTokens, c.CacheHitRate*100, c.TotalCostUSD)
		}
	}
	bySuiteCost := map[string]float64{}
	for _, run := range r.Runs {
		bySuiteCost[run.Suite+" / "+run.Model] += run.Metrics.CostUSD
	}
	if paid && len(bySuiteCost) > 0 {
		b.WriteString("\n| Suite / model | Spend |\n|---|---|\n")
		for _, k := range sortedKeys(bySuiteCost) {
			fmt.Fprintf(&b, "| %s | $%.2f |\n", k, bySuiteCost[k])
		}
	}
	if len(a.TopFailing) > 0 {
		b.WriteString("\n## Top failing graders\n\n")
		for _, g := range a.TopFailing {
			fmt.Fprintf(&b, "- %s: %d\n", g.Grader, g.Count)
		}
	}
	worst := worstCases(a)
	if len(worst) > 0 {
		b.WriteString("\n## Worst cases\n\n| Case | Model | Arm | Pass |\n|---|---|---|---|\n")
		for _, k := range worst {
			cs, m, arm := split3(k)
			v := a.Cases[k]
			fmt.Fprintf(&b, "| %s | %s | %s | %d/%d |\n", cs, m, arm, v.Pass, v.Total)
		}
	}
	b.WriteString("\n## Runs\n\n| Case | Arm | Model | Run | Status | Calls |")
	if paid {
		b.WriteString(" Cost |")
	}
	b.WriteString(" First failure | Transcript |\n|---|---|---|---|---|---|")
	if paid {
		b.WriteString("---|")
	}
	b.WriteString("---|---|\n")
	for _, run := range r.Runs {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %s | %d |", run.Case, run.Arm, run.Model, run.RunIdx, run.Status, run.Metrics.ToolCalls)
		if paid {
			fmt.Fprintf(&b, " $%.3f |", run.Metrics.CostUSD)
		}
		fmt.Fprintf(&b, " %s | %s |\n", mdEscape(firstFailure(run)), orDash(run.Transcript))
	}
	return b.String()
}

func firstFailure(r *runner.Result) string {
	if r.Error != "" {
		return clip(r.Error, 140)
	}
	for _, g := range r.Graders {
		if g.Gating && !g.Skipped && !g.Pass && len(g.Failures) > 0 {
			return g.Grader + ": " + clip(g.Failures[0], 140)
		}
	}
	return ""
}

func worstCases(a Aggregates) []string {
	var keys []string
	for k, v := range a.Cases {
		if v.Rate < 1 {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if a.Cases[keys[i]].Rate != a.Cases[keys[j]].Rate {
			return a.Cases[keys[i]].Rate < a.Cases[keys[j]].Rate
		}
		return keys[i] < keys[j]
	})
	if len(keys) > 15 {
		keys = keys[:15]
	}
	return keys
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return suiteRank(keys[i]) < suiteRank(keys[j]) || suiteRank(keys[i]) == suiteRank(keys[j]) && keys[i] < keys[j]
	})
	return keys
}

func suiteRank(k string) int {
	for i, s := range catalog.Suites {
		if strings.HasPrefix(k, s) {
			return i
		}
	}
	return len(catalog.Suites)
}

func mdEscape(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func short(h string) string {
	if len(h) > 10 {
		return "`" + h[:10] + "`"
	}
	return "`" + h + "`"
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
