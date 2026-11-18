package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// templateRe matches {{seed.x.y}}, {{env.url}}, {{now}}; {{last.x}} and
// {{calls.cNN.x}} are runtime templates the fake model resolves, so they are
// left in place here.
var templateRe = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-\[\]]+)\s*\}\}`)

func isRuntime(expr string) bool {
	return expr == "last" || strings.HasPrefix(expr, "last.") || strings.HasPrefix(expr, "calls.")
}

// Lookup resolves a dotted path in vars ("seed.slowRequest.id").
func Lookup(vars map[string]any, expr string) (any, bool) {
	var cur any = vars
	for _, seg := range strings.Split(strings.NewReplacer("[", ".", "]", "").Replace(expr), ".") {
		if seg == "" {
			continue
		}
		switch t := cur.(type) {
		case map[string]any:
			v, ok := t[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			cur = t[i]
		default:
			return nil, false
		}
	}
	return cur, cur != nil
}

// ExpandString replaces templates in s. Unknown templates are an error;
// runtime templates are kept.
func ExpandString(s string, vars map[string]any) (string, error) {
	var firstErr error
	out := templateRe.ReplaceAllStringFunc(s, func(m string) string {
		expr := templateRe.FindStringSubmatch(m)[1]
		if isRuntime(expr) {
			return m
		}
		v, ok := Lookup(vars, expr)
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("template {{%s}} has no value", expr)
			}
			return m
		}
		return scalarString(v)
	})
	return out, firstErr
}

func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any, []any:
		raw, _ := json.Marshal(t)
		return string(raw)
	default:
		return fmt.Sprint(t)
	}
}

// Resolve returns a copy of the case with every template filled from vars.
// It re-reads the YAML and expands scalar nodes, so a scalar that is exactly
// one template keeps the value's type (a number stays a number).
func (c *Case) Resolve(vars map[string]any) (*Case, error) {
	raw, err := os.ReadFile(c.Path)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var firstErr error
	var walk func(n *yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && strings.Contains(n.Value, "{{") {
			trimmed := strings.TrimSpace(n.Value)
			if m := templateRe.FindStringSubmatch(trimmed); m != nil && m[0] == trimmed && !isRuntime(m[1]) {
				if v, ok := Lookup(vars, m[1]); ok {
					switch t := v.(type) {
					case float64:
						n.Value, n.Tag, n.Style = strconv.FormatFloat(t, 'f', -1, 64), "!!float", 0
					case int:
						n.Value, n.Tag, n.Style = strconv.Itoa(t), "!!int", 0
					case bool:
						n.Value, n.Tag, n.Style = strconv.FormatBool(t), "!!bool", 0
					default:
						n.Value, n.Tag = scalarString(v), "!!str"
					}
					return
				}
			}
			s, err := ExpandString(n.Value, vars)
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", c.ID, err)
			}
			n.Value = s
			if n.Tag != "!!str" && strings.Contains(s, "{{") {
				n.Tag = "!!str"
			}
		}
		for _, ch := range n.Content {
			walk(ch)
		}
	}
	walk(&doc)
	if firstErr != nil {
		return nil, firstErr
	}
	var out Case
	if err := doc.Decode(&out); err != nil {
		return nil, fmt.Errorf("%s: after templating: %w", c.ID, err)
	}
	out.Path = c.Path
	if out.Invoke == "" {
		out.Invoke = "natural"
	}
	return &out, nil
}
