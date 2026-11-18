package grade

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// Answer grades the structured submission against expect.answer.
func Answer(in *Input) *Verdict {
	v := newVerdict("answer", true)
	exp := in.Case.Expect.Answer
	sub := in.Run.Submission
	if sub == nil {
		v.fail("no submission (%s)", in.Run.StopReason)
		return v
	}
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return strings.TrimSpace(s) }
	switch in.Case.Suite {
	case "perf":
		got := str(sub, "checklist_item")
		want := append([]string{}, exp.AnyOf...)
		if exp.ChecklistItem != "" {
			want = append(want, exp.ChecklistItem)
		}
		if len(want) > 0 && !slices.Contains(want, got) {
			v.fail("checklist_item %q, want %s", got, strings.Join(want, " or "))
		}
		if slices.Contains(exp.Not, got) {
			v.fail("checklist_item %q is the distractor", got)
		}
		if exp.Locus != nil {
			locus, _ := sub["locus"].(map[string]any)
			if locus == nil {
				locus = map[string]any{}
			}
			if exp.Locus.Endpoint != "" && !answerEndpoint(str(locus, "endpoint"), exp.Locus.Endpoint) {
				v.fail("locus.endpoint %q, want %q", str(locus, "endpoint"), exp.Locus.Endpoint)
			}
			if exp.Locus.SpanContains != "" && !containsFold(str(locus, "span"), exp.Locus.SpanContains) {
				v.fail("locus.span %q does not contain %q", str(locus, "span"), exp.Locus.SpanContains)
			}
			if exp.Locus.Service != "" && !containsFold(str(locus, "service"), exp.Locus.Service) {
				v.fail("locus.service %q, want %q", str(locus, "service"), exp.Locus.Service)
			}
			if exp.Locus.Host != "" && !containsFold(str(locus, "host"), exp.Locus.Host) {
				v.fail("locus.host %q, want %q", str(locus, "host"), exp.Locus.Host)
			}
		}
		if exp.Onset != nil && exp.Onset.At != "" {
			gradeOnset(v, sub["onset"], exp.Onset)
		}
	case "debug":
		answerDebug(v, in, exp, sub)
	}
	if len(exp.MentionsAll) > 0 {
		raw, _ := json.Marshal(sub)
		text := string(raw) + "\n" + in.Run.FinalText
		for _, pat := range exp.MentionsAll {
			re, err := regexp.Compile("(?is)" + pat)
			if err != nil {
				v.fail("bad mentions_all pattern %q: %v", pat, err)
				continue
			}
			if !re.MatchString(text) {
				v.fail("the answer never mentions /%s/", pat)
			}
		}
	}
	if p, ok := sub["_validation_problems"].(string); ok {
		v.note("accepted after failed repairs: %s", p)
	}
	return v
}

func gradeOnset(v *Verdict, got any, exp *catalog.Onset) {
	want, err := time.Parse(time.RFC3339Nano, exp.At)
	if err != nil {
		v.fail("case onset %q is not RFC 3339", exp.At)
		return
	}
	s, _ := got.(string)
	at, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		v.fail("onset %v is not an RFC 3339 time", got)
		return
	}
	tol := time.Duration(exp.ToleranceMinutes) * time.Minute
	if tol == 0 {
		tol = time.Hour
	}
	if d := at.Sub(want); d < -tol || d > tol {
		v.fail("onset %s is %s from %s (tolerance %s)", at.Format(time.RFC3339), d.Round(time.Minute), want.Format(time.RFC3339), tol)
	}
}

var spaceRe = regexp.MustCompile(`\s+`)

// sameEndpoint compares "GET /api/orders" forms, tolerating case, spacing,
// a trailing slash and a missing method when the route matches.
func sameEndpoint(got, want string) bool {
	norm := func(s string) string {
		s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
		s = strings.Trim(s, "`\"'")
		method, route, ok := strings.Cut(s, " ")
		if !ok {
			return strings.TrimSuffix(s, "/")
		}
		return strings.ToUpper(method) + " " + strings.TrimSuffix(route, "/")
	}
	g, w := norm(got), norm(want)
	if g == w {
		return true
	}
	// The answer may leave the method out, or put a word before the
	// endpoint ("endpoint POST /api/checkout"); it may not name another
	// method.
	_, wr, _ := strings.Cut(w, " ")
	return g == wr || strings.HasSuffix(g, " "+w)
}

// answerEndpoint is sameEndpoint for an answer, which may qualify the right
// endpoint: "GET /api/me (and every route behind the same guard)". Text after
// the route is ignored; the route itself must still match.
func answerEndpoint(got, want string) bool {
	return sameEndpoint(got, want) || sameEndpoint(withoutQualifier(got), want)
}

// withoutQualifier cuts an endpoint answer after its route, the first field
// that starts with "/", and at a "(" inside that field.
func withoutQualifier(s string) string {
	fields := strings.Fields(s)
	for i, f := range fields {
		if strings.HasPrefix(strings.Trim(f, "`\"'"), "/") {
			route, _, _ := strings.Cut(f, "(")
			return strings.Join(append(fields[:i:i], route), " ")
		}
	}
	return s
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// Report checks free-text requirements on the final report.
func Report(in *Input) *Verdict {
	v := newVerdict("report", true)
	text := FinalText(in.Run)
	for _, pat := range in.Case.Expect.Report.Contains {
		if re, err := regexp.Compile("(?is)" + pat); err != nil || !re.MatchString(text) {
			v.fail("report does not match /%s/", pat)
		}
	}
	for _, pat := range in.Case.Expect.Report.NotContains {
		if re, err := regexp.Compile("(?is)" + pat); err == nil && re.MatchString(text) {
			v.fail("report must not match /%s/", pat)
		}
	}
	if text == "" {
		v.fail("no report")
	}
	return v
}
