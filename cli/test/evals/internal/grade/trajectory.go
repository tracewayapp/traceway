package grade

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// Trajectory checks the tool calls against the case's call DSL:
//
//	required   each matcher matches at least one operation
//	any_of     at least one group is fully satisfied
//	forbidden  no operation matches
//	order      the first match of A precedes the first match of B
//
// The thrash limits (max_calls, max_failed_calls, max_identical_repeats) are
// reported by Efficiency instead: checking a JSON shape twice is not a wrong
// answer. The case budget still stops a run that spins.
//
// An op pattern is an exact op, "*", a prefix ("endpoints.*"), or a file op
// with a path glob ("edit_file:handlers/*.go").
//
// required and any_of ask what the agent saw, so a read that failed does not
// satisfy them unless the matcher sets ok. A write counts when it was
// attempted: the policy cases expect a refused one.
func Trajectory(run *record.Run, t *catalog.Trajectory) *Verdict {
	v := newVerdict("trajectory", true)
	all := run.AllOps()
	for _, m := range t.Required {
		if firstDone(all, m) < 0 {
			v.fail("required call missing: %s", describe(m))
		}
	}
	if len(t.AnyOf) > 0 {
		ok := false
		for _, group := range t.AnyOf {
			hit := true
			for _, m := range group {
				if firstDone(all, m) < 0 {
					hit = false
					break
				}
			}
			if hit {
				ok = true
				break
			}
		}
		if !ok {
			var groups []string
			for _, g := range t.AnyOf {
				var parts []string
				for _, m := range g {
					parts = append(parts, describe(m))
				}
				groups = append(groups, "["+strings.Join(parts, " + ")+"]")
			}
			v.fail("none of these alternatives happened: %s", strings.Join(groups, " or "))
		}
	}
	for _, m := range t.Forbidden {
		if i := firstMatch(all, m); i >= 0 {
			v.fail("forbidden call %s happened in %s: %s", describe(m), all[i].Call.ID, opString(all[i].OpRecord))
		}
	}
	for _, pair := range t.Order {
		if len(pair) != 2 {
			v.fail("order entries need exactly two ops, got %v", pair)
			continue
		}
		a := firstMatch(all, catalog.CallMatcher{Op: pair[0]})
		b := firstMatch(all, catalog.CallMatcher{Op: pair[1]})
		switch {
		case b < 0:
		case a < 0:
			v.fail("%s happened without %s before it", pair[1], pair[0])
		case a > b:
			v.fail("%s (%s) came after %s (%s)", pair[0], all[a].Call.ID, pair[1], all[b].Call.ID)
		}
	}
	return v
}

// Efficiency checks the thrash limits. It does not gate: a run that repeats
// or retries calls still passes on its answer, evidence and writes, and the
// limits it went over are listed here for the report.
func Efficiency(run *record.Run, t *catalog.Trajectory) *Verdict {
	v := newVerdict("efficiency", false)
	all := run.AllOps()
	executed := 0
	failed := 0
	for _, c := range run.Calls {
		if c.Rejected || tools.IsSubmit(c.Tool) {
			continue
		}
		executed++
		if !c.OK || !anyOpOK(c) {
			failed++
		}
	}
	if t.MaxCalls > 0 && executed > t.MaxCalls {
		v.fail("%d tool calls, more than max_calls %d", executed, t.MaxCalls)
	}
	if t.MaxFailedCalls > 0 && failed > t.MaxFailedCalls {
		v.fail("%d failed tool calls, more than max_failed_calls %d", failed, t.MaxFailedCalls)
	}
	if t.MaxIdenticalRepeats > 0 {
		// Thrashing is asking the same question again. Changing a local file
		// (several edits, or appending to one file) is not, and neither is
		// one shell loop that runs the same command on each iteration: an
		// operation counts once per tool call.
		counts := map[string]int{}
		for _, c := range run.Calls {
			if c.Rejected {
				continue
			}
			seen := map[string]bool{}
			for i := range c.Ops {
				op := &c.Ops[i]
				if op.Op.Op == ops.OpWriteFile || op.Op.Op == ops.OpEditFile {
					continue
				}
				if k := opKey(op); !seen[k] {
					seen[k] = true
					counts[k]++
				}
			}
		}
		keys := make([]string, 0, len(counts))
		for k := range counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if counts[k] > t.MaxIdenticalRepeats {
				v.fail("the same call ran %d times (max_identical_repeats %d): %s", counts[k], t.MaxIdenticalRepeats, k)
			}
		}
	}
	v.note("%d operations in %d calls (%d failed)", len(all), executed, failed)
	return v
}

// firstDone is firstMatch for required and any_of: without an explicit ok
// in the matcher, only a successful read or an attempted write counts.
func firstDone(all []record.CallOp, m catalog.CallMatcher) int {
	for i, op := range all {
		if m.OK == nil && !op.OK && !op.Write {
			continue
		}
		if matches(op.OpRecord, m) {
			return i
		}
	}
	return -1
}

func firstMatch(all []record.CallOp, m catalog.CallMatcher) int {
	for i, op := range all {
		if matches(op.OpRecord, m) {
			return i
		}
	}
	return -1
}

// Matches reports whether an operation satisfies a matcher.
func Matches(op *record.OpRecord, m catalog.CallMatcher) bool { return matches(op, m) }

func matches(op *record.OpRecord, m catalog.CallMatcher) bool {
	pattern, glob, _ := strings.Cut(m.Op, ":")
	if !opPattern(pattern, op.Op.Op) {
		return false
	}
	if glob != "" {
		var target string
		if p, ok := op.Args["path"].(string); ok {
			target = p
		} else if n, ok := op.Args["name"].(string); ok {
			target = n
		}
		if ok, _ := path.Match(glob, target); !ok && !strings.HasSuffix(target, strings.TrimPrefix(glob, "*")) {
			return false
		}
	}
	if m.Write != nil && op.Write != *m.Write {
		return false
	}
	if m.OK != nil && op.OK != *m.OK {
		return false
	}
	for k, want := range m.Args {
		got, present := op.Args[k]
		if !argMatches(got, present, want) {
			return false
		}
	}
	return true
}

func opPattern(p, op string) bool {
	switch {
	case p == "*" || p == "":
		return true
	case strings.HasSuffix(p, ".*"):
		return strings.HasPrefix(op, strings.TrimSuffix(p, "*"))
	default:
		return p == op
	}
}

// argMatches implements the arg matchers: present, absent, ~regex, exact
// value, and {min, max}.
func argMatches(got any, present bool, want any) bool {
	switch w := want.(type) {
	case string:
		switch {
		case w == "present":
			return present && fmt.Sprint(got) != ""
		case w == "absent":
			return !present
		case strings.HasPrefix(w, "~"):
			re, err := regexp.Compile(w[1:])
			return err == nil && present && re.MatchString(argString(got))
		default:
			return present && argString(got) == w
		}
	case map[string]any:
		n, ok := toFloat(got)
		if !ok {
			return false
		}
		if lo, ok := toFloat(w["min"]); ok && n < lo {
			return false
		}
		if hi, ok := toFloat(w["max"]); ok && n > hi {
			return false
		}
		return present
	case bool:
		b, _ := strconv.ParseBool(argString(got))
		if gb, ok := got.(bool); ok {
			b = gb
		}
		return present && b == w
	default:
		return present && argString(got) == argString(w)
	}
}

func argString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case []any:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = argString(x)
		}
		return strings.Join(parts, ",")
	case map[string]any:
		raw, _ := json.Marshal(t)
		return string(raw)
	default:
		return fmt.Sprint(t)
	}
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

func describe(m catalog.CallMatcher) string {
	s := m.Op
	if s == "" {
		s = "*"
	}
	var parts []string
	keys := make([]string, 0, len(m.Args))
	for k := range m.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, m.Args[k]))
	}
	if m.Write != nil {
		parts = append(parts, fmt.Sprintf("write=%v", *m.Write))
	}
	if len(parts) > 0 {
		s += "(" + strings.Join(parts, ", ") + ")"
	}
	return s
}

func opString(op *record.OpRecord) string {
	if len(op.Argv) > 0 {
		return strings.Join(op.Argv, " ")
	}
	raw, _ := json.Marshal(op.Args)
	return op.Op.Op + " " + string(raw)
}

// opKey identifies a call for repeat detection, ignoring output-format flags.
// A git op records only its subcommand, so its argv is the key: three
// `git show` calls of different commits are three questions.
func opKey(op *record.OpRecord) string {
	if op.Op.Op == ops.OpGit && len(op.Argv) > 0 {
		return op.Op.Op + " " + strings.Join(op.Argv, " ")
	}
	args := map[string]any{}
	for k, v := range op.Args {
		if k == "output" || k == "fields" {
			continue
		}
		args[k] = v
	}
	raw, _ := json.Marshal(args)
	return op.Op.Op + " " + string(raw)
}
