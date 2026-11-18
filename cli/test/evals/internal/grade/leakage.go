package grade

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/checklist"
)

// Leakage lint: the seeded data and the prompt must not name the
// answer. Seeded strings (span names, attribute values, log bodies, stack
// traces, object names) are checked against a denylist of checklist terms
// and against the expected checklist row's title; prompts get the checklist
// terms only, since a user naturally says "slow" or "bug".

// seedDenylist are phrases that give a diagnosis away when they appear in
// telemetry. Real library messages never contain them.
var seedDenylist = []string{
	"n+1", "n + 1", "nplus1", "n_plus_1", "select n", "stampede", "thundering herd", "missing index", "unused index",
	"no index", "pool exhaust", "exhausted pool", "exhaustion", "gc pause", "noisy neighbo", "cold start", "warm-up",
	"warmup", "lock contention", "contention", "over-fetch", "overfetch", "no pagination", "unbounded", "fan-out",
	"fanout", "chatty", "staircase", "parallelize", "run in parallel", "retry storm", "aggressive retr", "connection reuse",
	"large payload", "payload too large", "low hit ratio", "cpu saturat", "saturation", "memory pressure",
	"disk bottleneck", "io bottleneck", "quadratic", "o(n^2)", "o(n2)", "o(n²)", "background task", "should be async",
	"excessive logging", "log spam", "root cause", "bottleneck", "slow query", "culprit", "the cause", "deliberately",
	"bug here", "intentional", "eval", "seeded", "distractor",
}

// promptDenylist are checklist phrases a prompt must not contain.
var promptDenylist = []string{
	"n+1", "n + 1", "stampede", "thundering herd", "missing index", "pool exhaust", "exhaustion", "gc pause",
	"garbage collect", "noisy neighbo", "cold start", "lock contention", "over-fetch", "pagination", "fan-out",
	"staircase", "in parallel", "retries", "connection reuse", "hit ratio", "saturat", "memory pressure",
	"quadratic", "o(n", "background task", "excessive logging", "culprit", "distractor",
}

// Finding is one leak.
type Finding struct {
	Where string
	Term  string
	Text  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s contains %q: %s", f.Where, f.Term, clipS(f.Text, 160))
}

// Lint checks a case's seeded strings and prompt.
func Lint(c *catalog.Case, seeded []string) []Finding {
	var out []Finding
	seen := map[string]bool{}
	terms := append([]string{}, seedDenylist...)
	if c.Expect.Answer != nil && c.Expect.Answer.ChecklistItem != "" {
		for _, r := range checklist.Rows() {
			if r.ID == c.Expect.Answer.ChecklistItem {
				terms = append(terms, strings.ToLower(r.Title))
			}
		}
	}
	for _, s := range seeded {
		low := strings.ToLower(s)
		for _, t := range terms {
			if containsTerm(low, t) {
				k := t + "|" + s
				if !seen[k] {
					seen[k] = true
					out = append(out, Finding{Where: "seed", Term: t, Text: s})
				}
			}
		}
	}
	low := strings.ToLower(c.Prompt)
	for _, t := range promptDenylist {
		if containsTerm(low, t) {
			out = append(out, Finding{Where: "prompt", Term: t, Text: c.Prompt})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

var wordCache sync.Map // term -> *regexp.Regexp

// containsTerm matches a term on word boundaries when it starts and ends
// with a letter, so "eval" does not match "evaluate" but does match "eval".
func containsTerm(s, term string) bool {
	if !strings.Contains(s, term) {
		return false
	}
	isWord := func(b byte) bool { return b >= 'a' && b <= 'z' }
	if !isWord(term[0]) || !isWord(term[len(term)-1]) {
		return true
	}
	re, ok := wordCache.Load(term)
	if !ok {
		re, _ = wordCache.LoadOrStore(term, regexp.MustCompile(`\b`+regexp.QuoteMeta(term)+`\b`))
	}
	return re.(*regexp.Regexp).MatchString(s)
}

func clipS(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
