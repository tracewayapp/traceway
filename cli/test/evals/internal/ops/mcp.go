package ops

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MCPPrefix is how the agent sees the bridged MCP tools, as Claude Code does.
const MCPPrefix = "mcp__traceway__"

// ParseMCP maps an MCP tool call to an op. name may carry MCPPrefix.
func ParseMCP(name string, args map[string]any) Op {
	initOps()
	name = strings.TrimPrefix(name, MCPPrefix)
	switch name {
	case "list_resources":
		return Op{Op: OpListRes}
	case "read_resource":
		return Op{Op: OpReadResource, Args: args}
	}
	cands := byMCP[name]
	if len(cands) == 0 {
		return Op{Op: "mcp." + name, Args: args}
	}
	spec := cands[0]
	out := map[string]any{}
	for k, v := range args {
		out[k] = v
	}
	switch name {
	case "set_notification_rule_enabled":
		if en, _ := out["enabled"].(bool); en {
			spec = byOp["alerts.enable"]
		} else {
			spec = byOp["alerts.disable"]
		}
		delete(out, "enabled")
	case "query_metrics":
		// Flatten the first query so CLI and MCP args line up; the full
		// list stays under "queries".
		if qs, ok := out["queries"].([]any); ok && len(qs) > 0 {
			if q, ok := qs[0].(map[string]any); ok {
				for k, v := range q {
					if _, exists := out[k]; !exists {
						out[k] = v
					}
				}
			}
		}
	}
	return Op{Op: spec.Op, Args: out, Write: spec.Write}
}

// RenderMCP renders an op as an MCP tool call (name without prefix, args).
func RenderMCP(op string, args map[string]any) (string, map[string]any, error) {
	spec, ok := SpecFor(op)
	if !ok || spec.MCP == "" {
		return "", nil, fmt.Errorf("op %q has no MCP tool", op)
	}
	out := map[string]any{}
	for k, v := range args {
		if k == "output" || k == "fields" || k == "yes" {
			continue
		}
		out[k] = v
	}
	switch op {
	case "alerts.enable":
		out["enabled"] = true
	case "alerts.disable":
		out["enabled"] = false
	case "metrics.query":
		q := map[string]any{}
		for _, k := range []string{"name", "aggregation", "group_by", "tag_filters"} {
			if v, ok := out[k]; ok {
				q[k] = v
				delete(out, k)
			}
		}
		if tf, ok := q["tag_filters"]; ok {
			q["tag_filters"] = tagMap(tf)
		}
		out["queries"] = []any{q}
	}
	for _, k := range []string{"id", "incident_id", "duration_minutes", "offset_ms", "interval_minutes", "page", "page_size", "min_severity"} {
		if v, ok := out[k]; ok {
			if n, ok := asInt(v); ok {
				out[k] = n
			}
		}
	}
	return spec.MCP, out, nil
}

func tagMap(v any) map[string]any {
	out := map[string]any{}
	var items []any
	switch t := v.(type) {
	case map[string]any:
		return t
	case []any:
		items = t
	default:
		items = []any{t}
	}
	for _, it := range items {
		k, val, _ := strings.Cut(fmt.Sprint(it), "=")
		out[k] = val
	}
	return out
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case float64:
		return int(t), true
	case json.Number:
		// The fake model's templates keep looked-up numbers as written.
		if n, err := t.Int64(); err == nil {
			return int(n), true
		}
	case string:
		var n int
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil && fmt.Sprint(n) == t {
			return n, true
		}
	}
	return 0, false
}
