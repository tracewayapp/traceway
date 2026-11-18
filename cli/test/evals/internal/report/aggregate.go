package report

import (
	"sort"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/runner"
)

// Rate is a pass count.
type Rate struct {
	Pass  int     `json:"pass"`
	Total int     `json:"total"`
	Rate  float64 `json:"rate"`
}

func (r *Rate) add(pass bool) {
	r.Total++
	if pass {
		r.Pass++
	}
	r.Rate = float64(r.Pass) / float64(r.Total)
}

// Delta compares two arms or models.
type Delta struct {
	Suite string  `json:"suite"`
	Model string  `json:"model,omitempty"`
	A     string  `json:"a"`
	B     string  `json:"b"`
	RateA float64 `json:"rateA"`
	RateB float64 `json:"rateB"`
	Delta float64 `json:"delta"` // A - B in percentage points
	N     int     `json:"n"`
}

// Costs aggregates spend on passing runs.
type Costs struct {
	PassingRuns       int     `json:"passingRuns"`
	AvgCostUSD        float64 `json:"avgCostUsd"`
	AvgToolCalls      float64 `json:"avgToolCalls"`
	AvgInputTokens    float64 `json:"avgInputTokens"`
	AvgOutputTokens   float64 `json:"avgOutputTokens"`
	TotalCostUSD      float64 `json:"totalCostUsd"`
	CacheHitRate      float64 `json:"cacheHitRate"`
	AllRunsAvgCostUSD float64 `json:"allRunsAvgCostUsd"`
	AllRuns           int     `json:"allRuns"`
}

// Aggregates is the summary block of results.json.
type Aggregates struct {
	// Cases is keyed "case|model|arm".
	Cases map[string]*Rate `json:"cases"`
	// Suites is keyed "suite|model|arm".
	Suites      map[string]*Rate  `json:"suites"`
	SkillUplift []Delta           `json:"skillUplift"`
	CLIvsMCP    []Delta           `json:"cliVsMcp"`
	Models      []Delta           `json:"models"`
	Cost        map[string]*Costs `json:"cost"` // by model
	SpendUSD    float64           `json:"spendUsd"`
	JudgeUSD    float64           `json:"judgeUsd"`
	Statuses    map[string]int    `json:"statuses"`
	TopFailing  []GraderCount     `json:"topFailingGraders"`
	// Efficiency is keyed "model|arm". It sits beside the pass rate and is
	// never part of it.
	Efficiency map[string]*Effort `json:"efficiency"`
}

// Effort is how hard the runs of one model and arm worked.
type Effort struct {
	Runs           int     `json:"runs"`
	AvgToolCalls   float64 `json:"avgToolCalls"`
	AvgFailedCalls float64 `json:"avgFailedCalls"`
	// OverLimit counts runs that went over a case's max_calls,
	// max_failed_calls or max_identical_repeats.
	OverLimit int `json:"overLimit"`
}

// GraderCount counts failing graders.
type GraderCount struct {
	Grader string `json:"grader"`
	Count  int    `json:"count"`
}

// Counted reports whether a run counts toward pass rates: infra errors and
// cost-ceiling skips are excluded, budget exhaustion is a failure.
func Counted(r *runner.Result) bool {
	return r.Status != "infra_error" && r.Status != "cost_ceiling"
}

// Aggregate computes pass rates and deltas.
func Aggregate(runs []*runner.Result) Aggregates {
	a := Aggregates{Cases: map[string]*Rate{}, Suites: map[string]*Rate{}, Cost: map[string]*Costs{}, Statuses: map[string]int{}, Efficiency: map[string]*Effort{}}
	graderFails := map[string]int{}
	cacheIn, cacheHit := map[string]int{}, map[string]int{}
	for _, r := range runs {
		a.Statuses[r.Status]++
		a.SpendUSD += r.Metrics.CostUSD + r.Metrics.JudgeCostUSD
		a.JudgeUSD += r.Metrics.JudgeCostUSD
		c := a.Cost[r.Model]
		if c == nil {
			c = &Costs{}
			a.Cost[r.Model] = c
		}
		c.AllRuns++
		c.AllRunsAvgCostUSD += r.Metrics.CostUSD
		c.TotalCostUSD += r.Metrics.CostUSD
		cacheIn[r.Model] += r.Metrics.InputTokens
		cacheHit[r.Model] += r.Metrics.CachedTokens
		if !Counted(r) {
			continue
		}
		pass := r.Status == "pass"
		key := r.Case + "|" + r.Model + "|" + r.Arm
		if a.Cases[key] == nil {
			a.Cases[key] = &Rate{}
		}
		a.Cases[key].add(pass)
		skey := r.Suite + "|" + r.Model + "|" + r.Arm
		if a.Suites[skey] == nil {
			a.Suites[skey] = &Rate{}
		}
		a.Suites[skey].add(pass)
		if pass {
			c.PassingRuns++
			c.AvgCostUSD += r.Metrics.CostUSD
			c.AvgToolCalls += float64(r.Metrics.ToolCalls)
			c.AvgInputTokens += float64(r.Metrics.InputTokens)
			c.AvgOutputTokens += float64(r.Metrics.OutputTokens)
		}
		ekey := r.Model + "|" + r.Arm
		e := a.Efficiency[ekey]
		if e == nil {
			e = &Effort{}
			a.Efficiency[ekey] = e
		}
		e.Runs++
		e.AvgToolCalls += float64(r.Metrics.ToolCalls)
		e.AvgFailedCalls += float64(r.Metrics.FailedCalls)
		for _, g := range r.Graders {
			if g.Gating && !g.Skipped && !g.Pass {
				graderFails[g.Grader]++
			}
			if g.Grader == "efficiency" && !g.Pass {
				e.OverLimit++
			}
		}
	}
	for _, e := range a.Efficiency {
		e.AvgToolCalls /= float64(e.Runs)
		e.AvgFailedCalls /= float64(e.Runs)
	}
	for m, c := range a.Cost {
		if c.PassingRuns > 0 {
			n := float64(c.PassingRuns)
			c.AvgCostUSD /= n
			c.AvgToolCalls /= n
			c.AvgInputTokens /= n
			c.AvgOutputTokens /= n
		}
		if c.AllRuns > 0 {
			c.AllRunsAvgCostUSD /= float64(c.AllRuns)
		}
		if cacheIn[m] > 0 {
			c.CacheHitRate = float64(cacheHit[m]) / float64(cacheIn[m])
		}
	}
	for g, n := range graderFails {
		a.TopFailing = append(a.TopFailing, GraderCount{g, n})
	}
	sort.Slice(a.TopFailing, func(i, j int) bool { return a.TopFailing[i].Count > a.TopFailing[j].Count })

	// Deltas are computed on cases run on both sides, so a partial
	// selection does not compare different case sets.
	pair := func(suite, model, armA, armB string) (Delta, bool) {
		var ra, rb Rate
		for key, r := range a.Cases {
			cs, m, arm := split3(key)
			if m != model || suiteOf(cs) != suite || arm != armA {
				continue
			}
			other := a.Cases[cs+"|"+model+"|"+armB]
			if other == nil {
				continue
			}
			ra.Pass, ra.Total = ra.Pass+r.Pass, ra.Total+r.Total
			rb.Pass, rb.Total = rb.Pass+other.Pass, rb.Total+other.Total
		}
		if ra.Total == 0 || rb.Total == 0 {
			return Delta{}, false
		}
		pa, pb := float64(ra.Pass)/float64(ra.Total), float64(rb.Pass)/float64(rb.Total)
		return Delta{Suite: suite, Model: model, A: armA, B: armB, RateA: pa, RateB: pb, Delta: (pa - pb) * 100, N: ra.Total}, true
	}
	models := map[string]bool{}
	for _, r := range runs {
		models[r.Model] = true
	}
	var modelList []string
	for m := range models {
		modelList = append(modelList, m)
	}
	sort.Strings(modelList)
	for _, s := range catalog.Suites {
		for _, m := range modelList {
			if d, ok := pair(s, m, catalog.ArmCLISkill, catalog.ArmCLINoSkill); ok {
				a.SkillUplift = append(a.SkillUplift, d)
			}
			if d, ok := pair(s, m, catalog.ArmCLISkill, catalog.ArmMCP); ok {
				a.CLIvsMCP = append(a.CLIvsMCP, d)
			}
			if d, ok := pair(s, m, catalog.ArmCLIWriteSkill, catalog.ArmMCPWrite); ok {
				a.CLIvsMCP = append(a.CLIvsMCP, d)
			}
		}
		// Model vs model on each arm, over the cases both ran.
		for i := 0; i < len(modelList); i++ {
			for j := i + 1; j < len(modelList); j++ {
				for _, arm := range catalog.AllArms {
					var ra, rb Rate
					for key, r := range a.Cases {
						cs, m, ar := split3(key)
						if m != modelList[i] || ar != arm || suiteOf(cs) != s {
							continue
						}
						if o := a.Cases[cs+"|"+modelList[j]+"|"+arm]; o != nil {
							ra.Pass, ra.Total = ra.Pass+r.Pass, ra.Total+r.Total
							rb.Pass, rb.Total = rb.Pass+o.Pass, rb.Total+o.Total
						}
					}
					if ra.Total > 0 && rb.Total > 0 {
						pa, pb := float64(ra.Pass)/float64(ra.Total), float64(rb.Pass)/float64(rb.Total)
						a.Models = append(a.Models, Delta{Suite: s, A: modelList[i] + " " + arm, B: modelList[j] + " " + arm, RateA: pa, RateB: pb, Delta: (pa - pb) * 100, N: ra.Total})
					}
				}
			}
		}
	}
	return a
}

func split3(k string) (string, string, string) {
	var parts [3]string
	i := 0
	start := 0
	for p := 0; p < len(k) && i < 2; p++ {
		if k[p] == '|' {
			parts[i] = k[start:p]
			i++
			start = p + 1
		}
	}
	parts[2] = k[start:]
	return parts[0], parts[1], parts[2]
}

func suiteOf(caseID string) string {
	for _, s := range catalog.Suites {
		if len(caseID) > len(s) && caseID[:len(s)+1] == s+"-" {
			return s
		}
	}
	return ""
}
