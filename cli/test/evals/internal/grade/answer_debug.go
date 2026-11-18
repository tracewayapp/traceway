package grade

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
)

// answerDebug grades a submit_debug_diagnosis payload.
//
// issue_hash names the group the diagnosis is about. An empty field is
// accepted when the expected hash is in the prompt or anywhere in the
// answer: the model worked on that group and left a bookkeeping field
// blank. A different hash is a different group, and fails.
//
// culprit.line is read against the file before the agent's edit. A line
// that is off by more than the tolerance is mapped back through the edit
// (lineBeforeEdit), so a model that inserted a guard above the faulty
// statement and reported where it now sits is not marked wrong.
func answerDebug(v *Verdict, in *Input, exp *catalog.Answer, sub map[string]any) {
	str := func(m map[string]any, k string) string { s, _ := m[k].(string); return strings.TrimSpace(s) }
	if exp.Category != "" && str(sub, "category") != exp.Category {
		v.fail("category %q, want %q", str(sub, "category"), exp.Category)
	}
	if exp.IssueHash != "" {
		switch got := str(sub, "issue_hash"); {
		case strings.EqualFold(got, exp.IssueHash):
		case got == "" && hashNamed(in, exp.IssueHash):
			v.note("issue_hash left empty; %s is named in the prompt or the answer", exp.IssueHash)
		default:
			v.fail("issue_hash %q, want %q", got, exp.IssueHash)
		}
	}
	if exp.OccurrenceID != "" && !strings.EqualFold(str(sub, "occurrence_id"), exp.OccurrenceID) {
		v.fail("occurrence_id %q, want %q", str(sub, "occurrence_id"), exp.OccurrenceID)
	}
	if exp.Culprit != nil {
		c, _ := sub["culprit"].(map[string]any)
		if c == nil {
			c = map[string]any{}
		}
		if exp.Culprit.File != "" && !samePath(str(c, "file"), exp.Culprit.File) {
			v.fail("culprit.file %q, want %q", str(c, "file"), exp.Culprit.File)
		}
		if exp.Culprit.Symbol != "" && !sameSymbol(str(c, "symbol"), exp.Culprit.Symbol) {
			v.fail("culprit.symbol %q, want %q", str(c, "symbol"), exp.Culprit.Symbol)
		}
		if exp.Culprit.Line > 0 {
			tol := exp.Culprit.LineTolerance
			if tol == 0 {
				tol = 3
			}
			line, _ := toFloat(c["line"])
			got := int(line)
			if d := got - exp.Culprit.Line; d < -tol || d > tol {
				if before, ok := lineBeforeEdit(in, exp.Culprit.File, got); ok && before-exp.Culprit.Line >= -tol && before-exp.Culprit.Line <= tol {
					v.note("culprit.line %d is in the edited file; before the edit it was line %d", got, before)
				} else {
					v.fail("culprit.line %d, want %d±%d", got, exp.Culprit.Line, tol)
				}
			}
		}
	}
	if exp.NeedsInstrumentation != nil {
		got, _ := sub["needs_instrumentation"].(bool)
		if got != *exp.NeedsInstrumentation {
			v.fail("needs_instrumentation %v, want %v", got, *exp.NeedsInstrumentation)
		}
	}
}

// hashNamed reports whether the expected hash appears in the prompt, the
// submission or the final text.
func hashNamed(in *Input, hash string) bool {
	if in == nil || in.Run == nil {
		return false
	}
	raw, _ := json.Marshal(in.Run.Submission)
	text := strings.ToLower(in.Run.Prompt + "\n" + string(raw) + "\n" + in.Run.FinalText)
	return strings.Contains(text, strings.ToLower(hash))
}

// samePath compares a cited file with the expected one, accepting a path
// prefix, ./, backslashes, backticks and a trailing :line(:col).
func samePath(got, want string) bool {
	g := strings.Trim(strings.TrimSpace(got), "`")
	g = strings.ReplaceAll(g, "\\", "/")
	g = lineSuffixRe.ReplaceAllString(g, "")
	g = filepath.ToSlash(filepath.Clean(strings.TrimPrefix(g, "./")))
	w := filepath.ToSlash(filepath.Clean(want))
	return g == w || strings.HasSuffix(g, "/"+w) || strings.HasSuffix(w, "/"+g) && strings.Contains(g, "/")
}

var lineSuffixRe = regexp.MustCompile(`(:\d+){1,2}$`)

// sameSymbol accepts "(*OrderHandler).List", "OrderHandler.List", "List",
// "func List" or "List(ctx, id)" for an expected "List" (and the reverse).
func sameSymbol(got, want string) bool {
	last := func(s string) string {
		s = strings.Trim(strings.TrimSpace(s), "`")
		s = strings.TrimSpace(strings.TrimPrefix(s, "func "))
		// A trailing parameter list, but not the receiver of (*T).M.
		if strings.HasSuffix(s, ")") {
			if i := strings.LastIndex(s, "("); i > 0 {
				s = s[:i]
			}
		}
		// Swift accessor frames: chatURL.getter.
		for _, suf := range []string{".getter", ".setter", ".modify", ".init", ".deinit"} {
			s = strings.TrimSuffix(s, suf)
		}
		if i := strings.LastIndexAny(s, ".:#"); i >= 0 {
			s = s[i+1:]
		}
		return strings.ToLower(strings.Trim(s, "()* "))
	}
	return last(got) != "" && last(got) == last(want)
}
