package report

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// Baseline is the committed baseline.json: pass rates per suite and case,
// and measured per-run usage per model that the preflight uses instead of
// the models.yaml envelope.
type Baseline struct {
	GeneratedAt string                     `json:"generatedAt"`
	From        string                     `json:"from"`
	GitSHA      string                     `json:"gitSha"`
	Note        string                     `json:"note,omitempty"`
	Suites      map[string]*Rate           `json:"suites"` // "suite|model|arm"
	Cases       map[string]*Rate           `json:"cases"`  // "case|model|arm"
	PerRun      map[string]MeasuredUsage   `json:"perRun"` // by model
	Extra       map[string]json.RawMessage `json:"extra,omitempty"`
}

// MeasuredUsage is a model's average usage per run.
type MeasuredUsage struct {
	Runs         int     `json:"runs"`
	InputTokens  float64 `json:"inputTokens"`
	CachedTokens float64 `json:"cachedTokens"`
	OutputTokens float64 `json:"outputTokens"`
	CostUSD      float64 `json:"costUsd"`
	CostUSDP90   float64 `json:"costUsdP90"`
}

// LoadBaseline reads baseline.json.
func LoadBaseline(path string) (*Baseline, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &b, nil
}

// NewBaseline builds a baseline from results (twevals baseline update).
func NewBaseline(r *Results) *Baseline {
	a := Aggregate(r.Runs)
	b := &Baseline{GeneratedAt: time.Now().UTC().Format(time.RFC3339), From: r.Header.RunID, GitSHA: r.Header.GitSHA,
		Suites: a.Suites, Cases: a.Cases, PerRun: map[string]MeasuredUsage{}}
	costs := map[string][]float64{}
	sums := map[string]*MeasuredUsage{}
	for _, run := range r.Runs {
		if !Counted(run) || run.Model == "oracle" {
			continue
		}
		m := sums[run.Model]
		if m == nil {
			m = &MeasuredUsage{}
			sums[run.Model] = m
		}
		m.Runs++
		m.InputTokens += float64(run.Metrics.InputTokens)
		m.CachedTokens += float64(run.Metrics.CachedTokens)
		m.OutputTokens += float64(run.Metrics.OutputTokens)
		m.CostUSD += run.Metrics.CostUSD
		costs[run.Model] = append(costs[run.Model], run.Metrics.CostUSD)
	}
	for k, m := range sums {
		n := float64(m.Runs)
		m.InputTokens /= n
		m.CachedTokens /= n
		m.OutputTokens /= n
		m.CostUSD /= n
		cs := costs[k]
		sort.Float64s(cs)
		m.CostUSDP90 = cs[int(float64(len(cs)-1)*0.9)]
		b.PerRun[k] = *m
	}
	return b
}

// Save writes the baseline.
func (b *Baseline) Save(path string) error {
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// Regression is one finding of Compare.
type Regression struct {
	Kind   string  `json:"kind"` // suite | case
	Key    string  `json:"key"`
	Before float64 `json:"before"`
	After  float64 `json:"after"`
	Detail string  `json:"detail"`
}

// Compare applies the regression rule: a suite's pass rate drops
// more than 10 percentage points for a model × arm, or a case that passed at
// least 2 runs at a rate of 2 of 3 or better in the baseline now passes 0 of N
// (a single passing run is not enough history to call a single failure a
// regression).
func Compare(r *Results, b *Baseline) []Regression {
	a := Aggregate(r.Runs)
	var out []Regression
	for k, now := range a.Suites {
		before, ok := b.Suites[k]
		if !ok || before.Total == 0 {
			continue
		}
		if drop := (before.Rate - now.Rate) * 100; drop > 10 {
			out = append(out, Regression{Kind: "suite", Key: k, Before: before.Rate, After: now.Rate,
				Detail: fmt.Sprintf("%s pass rate fell %.0fpp (%.0f%% -> %.0f%%)", k, drop, before.Rate*100, now.Rate*100)})
		}
	}
	for k, now := range a.Cases {
		before, ok := b.Cases[k]
		if !ok || before.Total == 0 {
			continue
		}
		if before.Pass >= 2 && before.Rate >= 2.0/3.0-1e-9 && now.Pass == 0 && now.Total > 0 {
			out = append(out, Regression{Kind: "case", Key: k, Before: before.Rate, After: 0,
				Detail: fmt.Sprintf("%s passed %d/%d in the baseline and 0/%d now", k, before.Pass, before.Total, now.Total)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
