package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/checklist"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/record"
)

// The final-answer tools: perf, debug, and finish for write and setup.
const (
	SubmitPerf  = "submit_perf_diagnosis"
	SubmitDebug = "submit_debug_diagnosis"
	Finish      = "finish"
)

// IsSubmit reports whether a tool is a final-answer tool.
func IsSubmit(name string) bool {
	return name == SubmitPerf || name == SubmitDebug || name == Finish
}

// DebugCategories are submit_debug_diagnosis's category values.
var DebugCategories = []string{"null_reference", "missing_key", "type_or_contract_mismatch", "race_or_duplicate", "timeout", "config", "resource_exhaustion", "logic", "unknown"}

// Submit is a final-answer tool. It validates the payload against its
// schema and the call ids it cites; a failure goes back to the model for
// repair, up to MaxRepairs times, after which the last payload is accepted
// as-is for grading.
type Submit struct {
	Kind       string // SubmitPerf | SubmitDebug | Finish
	CallExists func(id string) (ok, exists bool)
	MaxRepairs int

	mu      sync.Mutex
	repairs int
}

func (t *Submit) Spec() Spec {
	switch t.Kind {
	case SubmitPerf:
		var rows []string
		for _, r := range checklist.Rows() {
			rows = append(rows, fmt.Sprintf("%s (%s)", r.ID, r.Title))
		}
		ids := checklist.IDs()
		item := map[string]any{"type": "string", "enum": ids}
		return Spec{Name: SubmitPerf, Description: "Submit your final performance diagnosis. This ends the task. " +
			"checklist_item is the bottleneck checklist row that explains the latency, or none_found when the latency is within expectations, or insufficient_evidence when the telemetry cannot tell. Rows: " + strings.Join(rows, "; ") + ". " +
			"Every evidence item cites the id of an earlier tool call (the [call cNN] line at the top of its result).",
			Parameters: obj(map[string]any{
				"checklist_item": item,
				"category":       strProp("database, application, network, caching, infrastructure, or none"),
				"locus": obj(map[string]any{
					"endpoint": strProp("Endpoint name, e.g. GET /api/orders"),
					"span":     strProp("The dominating span or query"),
					"service":  strProp("Service name"),
					"host":     strProp("Host or instance, when the cause is host-specific"),
				}),
				"onset":      map[string]any{"type": []string{"string", "null"}, "description": "RFC 3339 time the slowness started, or null"},
				"evidence":   evidenceSchema(),
				"ruled_out":  map[string]any{"type": "array", "items": obj(map[string]any{"checklist_item": item, "call_id": strProp("Call id"), "reason": strProp("Why it is ruled out")}, "checklist_item", "reason")},
				"fix":        strProp("The concrete fix"),
				"confidence": map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
				"report":     strProp("Your report to the user, in Markdown"),
			}, "checklist_item", "locus", "evidence", "fix", "confidence", "report")}
	case SubmitDebug:
		return Spec{Name: SubmitDebug, Description: "Submit your final debugging diagnosis. This ends the task. Every evidence item cites the id of an earlier tool call (the [call cNN] line at the top of its result).",
			Parameters: obj(map[string]any{
				"issue_hash":    strProp("The exception group hash (16 hex) of the issue you diagnosed. Set it whenever the issue has one: it is in the task text, in the /issues/<hash> URL, or in the exceptions list and show output. Leave it empty only when there is no exception group."),
				"occurrence_id": strProp("The occurrence you anchored on, if any"),
				"category":      map[string]any{"type": "string", "enum": DebugCategories},
				"culprit": obj(map[string]any{
					"file":   strProp("Workspace-relative file"),
					"symbol": strProp("Function or method"),
					"line":   intProp("Line number of the faulty statement, in the file as you found it; a line in the edited file is accepted too"),
				}, "file"),
				"evidence":              evidenceSchema(),
				"fix_summary":           strProp("What you changed or propose to change"),
				"files_changed":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"needs_instrumentation": map[string]any{"type": "boolean", "description": "True when the telemetry cannot explain the failure and you added instrumentation instead of a fix"},
				"confidence":            map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
				"report":                strProp("Your report to the user, in Markdown"),
			}, "category", "culprit", "evidence", "fix_summary", "needs_instrumentation", "confidence", "report")}
	default:
		return Spec{Name: Finish, Description: "Finish the task and report what you did. This ends the task.",
			Parameters: obj(map[string]any{
				"summary": strProp("Your report to the user, in Markdown"),
				"changes": map[string]any{"type": "array", "items": obj(map[string]any{
					"action":    strProp("What you changed, e.g. snooze alert rule"),
					"target":    strProp("The record, e.g. rule 12"),
					"new_value": strProp("The new state"),
					"undo":      strProp("How to undo it"),
				}, "action", "target")},
			}, "summary", "changes")}
	}
}

func evidenceSchema() map[string]any {
	return map[string]any{"type": "array", "minItems": 1, "items": obj(map[string]any{
		"call_id": strProp("Id of the tool call, e.g. c04"),
		"claim":   strProp("What that call showed"),
	}, "call_id", "claim")}
}

func (t *Submit) Call(_ context.Context, raw json.RawMessage) Result {
	var payload map[string]any
	if err := decodeArgs(raw, &payload); err != nil {
		return t.repair(nil, []string{err.Error()})
	}
	problems := t.validate(payload)
	if len(problems) > 0 {
		return t.repair(payload, problems)
	}
	return t.accept(payload, "")
}

func (t *Submit) repair(payload map[string]any, problems []string) Result {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.repairs++
	if t.repairs > t.MaxRepairs && payload != nil {
		return t.accept(payload, strings.Join(problems, "; "))
	}
	return Result{Output: "Your submission was not accepted. Fix these problems and call " + t.Kind + " again:\n- " + strings.Join(problems, "\n- "), OK: false,
		Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpSubmit, Args: map[string]any{"tool": t.Kind, "accepted": false}}}}}
}

func (t *Submit) accept(payload map[string]any, problems string) Result {
	msg := "Submission received."
	if problems != "" {
		payload["_validation_problems"] = problems
	}
	return Result{Output: msg, OK: true, Final: true, Submission: payload,
		Ops: []record.OpRecord{{Op: ops.Op{Op: ops.OpSubmit, Args: map[string]any{"tool": t.Kind, "accepted": true}}, OK: true}}}
}

func (t *Submit) validate(p map[string]any) []string {
	var probs []string
	str := func(k string) string { s, _ := p[k].(string); return s }
	need := func(keys ...string) {
		for _, k := range keys {
			if _, ok := p[k]; !ok {
				probs = append(probs, k+" is required")
			}
		}
	}
	checkEvidence := func() {
		ev, ok := p["evidence"].([]any)
		if !ok || len(ev) == 0 {
			probs = append(probs, "evidence must cite at least one tool call")
			return
		}
		for i, e := range ev {
			m, _ := e.(map[string]any)
			id, _ := m["call_id"].(string)
			if id == "" {
				probs = append(probs, fmt.Sprintf("evidence[%d].call_id is required", i))
				continue
			}
			if t.CallExists != nil {
				if _, exists := t.CallExists(id); !exists {
					probs = append(probs, fmt.Sprintf("evidence[%d].call_id %q is not a call in this session (ids look like c04)", i, id))
				}
			}
		}
	}
	switch t.Kind {
	case SubmitPerf:
		need("checklist_item", "locus", "evidence", "fix", "confidence", "report")
		if raw, present := p["locus"]; present {
			if _, ok := raw.(map[string]any); !ok {
				probs = append(probs, "locus must be an object with endpoint, span, service and host")
			}
		}
		if v := str("checklist_item"); v != "" && !checklist.IsID(v) {
			probs = append(probs, fmt.Sprintf("checklist_item %q is not one of the allowed values", v))
		}
		if rs, ok := p["ruled_out"].([]any); ok {
			for i, r := range rs {
				m, _ := r.(map[string]any)
				if v, _ := m["checklist_item"].(string); v != "" && !checklist.IsID(v) {
					probs = append(probs, fmt.Sprintf("ruled_out[%d].checklist_item %q is not an allowed value", i, v))
				}
			}
		}
		checkEvidence()
	case SubmitDebug:
		need("category", "culprit", "evidence", "fix_summary", "needs_instrumentation", "confidence", "report")
		if v := str("category"); v != "" && !slices.Contains(DebugCategories, v) {
			probs = append(probs, fmt.Sprintf("category %q is not one of %v", v, DebugCategories))
		}
		if raw, present := p["culprit"]; present {
			if c, ok := raw.(map[string]any); !ok {
				probs = append(probs, "culprit must be an object with file, symbol and line")
			} else if f, _ := c["file"].(string); f == "" {
				probs = append(probs, "culprit.file is required")
			}
		}
		checkEvidence()
	default:
		need("summary", "changes")
		if _, present := p["summary"]; present && strings.TrimSpace(str("summary")) == "" {
			probs = append(probs, "summary must be a non-empty string")
		}
		if raw, present := p["changes"]; present {
			if cs, ok := raw.([]any); !ok {
				probs = append(probs, "changes must be an array (empty when you changed nothing)")
			} else {
				for i, c := range cs {
					m, _ := c.(map[string]any)
					a, _ := m["action"].(string)
					tg, _ := m["target"].(string)
					if a == "" || tg == "" {
						probs = append(probs, fmt.Sprintf("changes[%d] needs an action and a target", i))
					}
				}
			}
		}
	}
	if c := str("confidence"); c != "" && !slices.Contains([]string{"low", "medium", "high"}, c) {
		probs = append(probs, "confidence must be low, medium or high")
	}
	for _, f := range markupFields(p, "") {
		probs = append(probs, f+" contains tool-call markup (<parameter name=...>); send every field as plain JSON")
	}
	return probs
}

// markupFields lists the string fields that carry tool-call markup: a model
// that serialized a nested object as its own tool-call syntax, which leaves
// the object a string and its fields flattened to the top level.
func markupFields(v any, at string) []string {
	var out []string
	switch t := v.(type) {
	case string:
		if strings.Contains(t, "<parameter name=") {
			out = append(out, at)
		}
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			name := k
			if at != "" {
				name = at + "." + k
			}
			out = append(out, markupFields(t[k], name)...)
		}
	case []any:
		for i, x := range t {
			out = append(out, markupFields(x, fmt.Sprintf("%s[%d]", at, i))...)
		}
	}
	return out
}
