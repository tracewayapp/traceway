package knowledge_test

import (
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

// jqOutputs is the type each command prints with --output json. An example
// piping a command that is missing here fails the test: add the command.
var jqOutputs = map[string]any{
	"endpoints list":     client.ListEndpointsResponse{},
	"endpoints requests": client.ListEndpointRequestsResponse{},
	"endpoints chart":    client.EndpointChartResponse{},
	"endpoints show":     client.EndpointDetailResponse{},
	"metrics query":      client.QueryMetricsResponse{},
	"exceptions list":    client.ListExceptionsResponse{},
	"exceptions show":    client.GetExceptionResponse{},
	"logs query":         client.QueryLogsResponse{},
}

// jqBuiltins are names a filter may use that are not output fields: the
// key/value pairs to_entries makes.
var jqBuiltins = map[string]bool{"key": true, "value": true}

var (
	exampleRe = regexp.MustCompile(`traceway(?:-write)? ([a-z-]+) ([a-z-]+)[^|'\n]*\|\s*jq(?: -[a-z]+)* '([^']*)'`)
	stringRe  = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	fieldRe   = regexp.MustCompile(`\.([A-Za-z_][A-Za-z0-9_]*)`)
	identRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	leadRe    = regexp.MustCompile(`^\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
)

// TestJQExamplesNameRealFields fails when a jq filter in a knowledge example
// reads a field the piped command's JSON does not have. Such a filter prints
// null, and an agent that copies it re-runs the command to find the real
// shape. A filter's first field must be a top-level field; the others are
// checked by name, at any depth of the output type.
func TestJQExamplesNameRealFields(t *testing.T) {
	examples := 0
	err := fs.WalkDir(knowledge.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".md") {
			return err
		}
		raw, err := knowledge.FS.ReadFile(path)
		if err != nil {
			return err
		}
		text := strings.ReplaceAll(string(raw), "\\\n", " ")
		for _, m := range exampleRe.FindAllStringSubmatch(text, -1) {
			examples++
			cmd, filter := m[1]+" "+m[2], m[3]
			out, ok := jqOutputs[cmd]
			if !ok {
				t.Errorf("%s: no output type for %q; add it to jqOutputs", path, cmd)
				continue
			}
			if lead := leadRe.FindStringSubmatch(filter); lead != nil && !jsonNames(reflect.TypeOf(out), nil)[lead[1]] {
				t.Errorf("%s: `%s | jq '%s'` starts at %q, which is not a top-level field of %s output", path, cmd, filter, lead[1], cmd)
			}
			names := jsonNames(reflect.TypeOf(out), map[reflect.Type]bool{})
			for _, f := range filterFields(filter) {
				if !names[f] && !jqBuiltins[f] {
					t.Errorf("%s: `%s | jq '%s'` reads %q, which %s output does not have", path, cmd, filter, f, cmd)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if examples == 0 {
		t.Fatal("found no jq examples; the pattern no longer matches the knowledge files")
	}
}

// filterFields returns the field names a jq filter reads: every .name, and
// every shorthand key of an object construction ({endpoint, count: .x}).
func filterFields(filter string) []string {
	f := stringRe.ReplaceAllString(filter, `""`)
	var out []string
	for _, m := range fieldRe.FindAllStringSubmatch(f, -1) {
		out = append(out, m[1])
	}
	for _, obj := range objectBodies(f) {
		for _, part := range splitTop(obj) {
			if p := strings.TrimSpace(part); identRe.MatchString(p) {
				out = append(out, p)
			}
		}
	}
	return out
}

// objectBodies returns the text inside each {...}, outermost first.
func objectBodies(s string) []string {
	var out []string
	var stack []int
	for i, r := range s {
		switch r {
		case '{':
			stack = append(stack, i)
		case '}':
			if n := len(stack); n > 0 {
				out = append(out, s[stack[n-1]+1:i])
				stack = stack[:n-1]
			}
		}
	}
	return out
}

// splitTop splits an object body on commas outside brackets and parentheses.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// jsonNames collects the JSON field names reachable from t, or only its
// top-level ones when seen is nil.
func jsonNames(t reflect.Type, seen map[reflect.Type]bool) map[string]bool {
	names := map[string]bool{}
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return names
	}
	if seen != nil {
		seen[t] = true
	}
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			for n := range jsonNames(f.Type, seen) {
				names[n] = true
			}
			continue
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		names[name] = true
		if seen == nil {
			continue
		}
		for n := range jsonNames(f.Type, seen) {
			names[n] = true
		}
	}
	return names
}
