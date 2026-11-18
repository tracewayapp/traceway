package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/catalog"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/ops"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/tools"
)

// Fake is an httptest server speaking Chat Completions that plays a case's
// oracle steps, one tool call per turn. It is the only model `go test` can
// reach. Surface-neutral `op:` steps are rendered as a CLI command on shell
// arms and as an MCP tool call on MCP arms, so one oracle covers both.
type Fake struct {
	Steps  []catalog.Step
	CLIBin string // traceway | traceway-write
	// FileRoot resolves write_file `from:` paths.
	FileRoot string

	srv  *httptest.Server
	mu   sync.Mutex
	errs []string
}

// NewFake starts the fake model.
func NewFake(steps []catalog.Step, cliBin, fileRoot string) *Fake {
	f := &Fake{Steps: steps, CLIBin: cliBin, FileRoot: fileRoot}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	return f
}

// Client returns a Chat Completions client pointed at the fake.
func (f *Fake) Client() *Client { return &Client{BaseURL: f.srv.URL, Retries: 1} }

// Close stops the server.
func (f *Fake) Close() { f.srv.Close() }

// Errors returns oracle rendering problems (a step naming a tool the arm
// does not have, a template that resolved to nothing, ...).
func (f *Fake) Errors() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.errs...)
}

func (f *Fake) fail(format string, a ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, fmt.Sprintf(format, a...))
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	step := 0
	for _, m := range req.Messages {
		if m.Role == "assistant" {
			step++
		}
	}
	tools := map[string]bool{}
	for _, t := range req.Tools {
		tools[t.Function.Name] = true
	}
	msg := Message{Role: "assistant"}
	if step >= len(f.Steps) {
		msg.Content = "Done."
	} else {
		name, args, text, err := f.render(f.Steps[step], tools, req.Messages)
		if err != nil {
			f.fail("step %d: %v", step+1, err)
			msg.Content = "oracle error: " + err.Error()
		} else if name == "" {
			msg.Content = text
		} else {
			raw, _ := json.Marshal(args)
			msg.ToolCalls = []ToolCall{{ID: fmt.Sprintf("fake_%d", step+1), Type: "function", Function: FunctionCall{Name: name, Arguments: string(raw)}}}
		}
	}
	resp := map[string]any{
		"id": fmt.Sprintf("fake-%d", step+1), "model": req.Model, "provider": "fake",
		"choices": []any{map[string]any{"message": msg, "finish_reason": "tool_calls"}},
		"usage":   map[string]any{"prompt_tokens": 0, "completion_tokens": 0, "cost": 0},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (f *Fake) render(s catalog.Step, tools map[string]bool, msgs []Message) (string, map[string]any, string, error) {
	tmpl := func(v string) (string, error) { return resolveRuntime(v, msgs) }
	mcpArm := false
	for name := range tools {
		if strings.HasPrefix(name, ops.MCPPrefix) {
			mcpArm = true
		}
	}
	switch {
	case s.Say != "":
		return "", nil, s.Say, nil
	case s.Shell != "":
		cmd, err := tmpl(s.Shell)
		if err != nil {
			return "", nil, "", err
		}
		if !mcpArm {
			return "shell", map[string]any{"command": cmd}, "", nil
		}
		op, err := firstTracewayOp(cmd)
		if err != nil {
			// Not a Traceway command (git, grep, ...): MCP arms have a
			// shell too, without the traceway CLI.
			if tools["shell"] {
				return "shell", map[string]any{"command": cmd}, "", nil
			}
			return "", nil, "", err
		}
		return f.mcpCall(op.Op, op.Args, tools)
	case s.Op != "":
		args, err := resolveArgs(s.Args, msgs)
		if err != nil {
			return "", nil, "", err
		}
		if mcpArm {
			if s.Stdin != "" {
				in, err := tmpl(s.Stdin)
				if err != nil {
					return "", nil, "", err
				}
				args["content_md"] = in
			}
			return f.mcpCall(s.Op, args, tools)
		}
		if _, ok := args["content_md"]; ok && s.Stdin == "" {
			s.Stdin = fmt.Sprint(args["content_md"])
			args["content_md"] = "-"
		}
		argv, err := ops.RenderCLI(s.Op, args)
		if err != nil {
			return "", nil, "", err
		}
		if ops.IsWriteOp(s.Op) {
			argv = append(argv, "--yes")
		}
		cmd := f.CLIBin + " " + shellJoin(argv)
		if s.Stdin != "" {
			in, err := tmpl(s.Stdin)
			if err != nil {
				return "", nil, "", err
			}
			cmd += " <<'TWEVALS_EOF'\n" + in + "\nTWEVALS_EOF"
		}
		return "shell", map[string]any{"command": cmd}, "", nil
	case s.ReadFile != "":
		return need(tools, "read_file", map[string]any{"path": s.ReadFile})
	case s.ListFiles != "":
		return need(tools, "list_files", map[string]any{"path": s.ListFiles})
	case s.Search != nil:
		return need(tools, "search", s.Search)
	case s.WriteFile != nil:
		content := s.WriteFile.Content
		if s.WriteFile.From != "" {
			raw, err := os.ReadFile(f.FileRoot + "/" + s.WriteFile.From)
			if err != nil {
				return "", nil, "", err
			}
			content = string(raw)
		}
		return need(tools, "write_file", map[string]any{"path": s.WriteFile.Path, "content": content})
	case s.EditFile != nil:
		return need(tools, "edit_file", map[string]any{"path": s.EditFile.Path, "old_string": s.EditFile.Old, "new_string": s.EditFile.New})
	case s.LoadSkill != "":
		if !tools["load_skill"] {
			// Arms without skills play the step as a harmless look at the
			// repository, so later call ids stay the same on every arm.
			return need(tools, "list_files", map[string]any{"path": "."})
		}
		return need(tools, "load_skill", map[string]any{"name": s.LoadSkill})
	case s.AskUser != "":
		q, err := tmpl(s.AskUser)
		if err != nil {
			return "", nil, "", err
		}
		return need(tools, "ask_user", map[string]any{"question": q})
	case s.Resource != "":
		return need(tools, ops.MCPPrefix+"read_resource", map[string]any{"uri": s.Resource})
	case s.Submit != nil:
		name := submitTool(tools)
		if name == "" {
			return "", nil, "", fmt.Errorf("no submit tool on this arm")
		}
		args, err := resolveArgs(s.Submit, msgs)
		return name, args, "", err
	case s.Finish != nil:
		args, err := resolveArgs(s.Finish, msgs)
		if err != nil {
			return "", nil, "", err
		}
		return need(tools, "finish", args)
	}
	return "", nil, "", fmt.Errorf("empty oracle step")
}

// submitTool names the arm's submit_* tool, the first by name so the choice
// never depends on map order. A finish step plays finish.
func submitTool(offered map[string]bool) string {
	var names []string
	for n := range offered {
		if tools.IsSubmit(n) && n != tools.Finish {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func need(tools map[string]bool, name string, args map[string]any) (string, map[string]any, string, error) {
	if !tools[name] {
		return "", nil, "", fmt.Errorf("this arm has no %s tool", name)
	}
	return name, args, "", nil
}

func (f *Fake) mcpCall(op string, args map[string]any, tools map[string]bool) (string, map[string]any, string, error) {
	name, margs, err := ops.RenderMCP(op, args)
	if err != nil {
		return "", nil, "", err
	}
	full := ops.MCPPrefix + name
	if !tools[full] {
		return "", nil, "", fmt.Errorf("this arm has no MCP tool %s", name)
	}
	return full, margs, "", nil
}

// firstTracewayOp parses a shell command and returns its first traceway
// invocation as an op.
func firstTracewayOp(cmd string) (ops.Op, error) {
	inv, err := ops.Invocations(cmd)
	if err != nil {
		return ops.Op{}, err
	}
	for _, words := range inv {
		if words[0] == "traceway" || words[0] == "traceway-write" {
			op := ops.ParseCLI(words[1:])
			delete(op.Args, "output")
			delete(op.Args, "yes")
			return op, nil
		}
	}
	return ops.Op{}, fmt.Errorf("shell step has no traceway invocation to translate for an MCP arm: %s", cmd)
}

func shellJoin(argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = shellQuote(a)
	}
	return strings.Join(out, " ")
}

var safeWord = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

func shellQuote(s string) string {
	if s != "" && safeWord.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

var runtimeRe = regexp.MustCompile(`\{\{\s*((?:last|calls\.c\d+)(?:\.[^}\s]+)?)\s*\}\}`)

// resolveRuntime fills {{last.x}} and {{calls.c03.x}} from the tool results
// already in the conversation.
func resolveRuntime(s string, msgs []Message) (string, error) {
	var firstErr error
	out := runtimeRe.ReplaceAllStringFunc(s, func(m string) string {
		expr := runtimeRe.FindStringSubmatch(m)[1]
		v, err := lookupRuntime(expr, msgs)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return runtimeText(v)
	})
	return out, firstErr
}

// runtimeText renders a looked-up value inside a string as it reads in the
// tool output: numbers as their JSON text (never 4.52e+07), objects as JSON.
func runtimeText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case map[string]any, []any:
		raw, _ := json.Marshal(t)
		return string(raw)
	default:
		return fmt.Sprint(t)
	}
}

func resolveArgs(in map[string]any, msgs []Message) (map[string]any, error) {
	out := map[string]any{}
	var firstErr error
	var walk func(v any) any
	walk = func(v any) any {
		switch t := v.(type) {
		case string:
			// A whole-string template keeps the looked-up value's type.
			if m := runtimeRe.FindStringSubmatch(t); m != nil && m[0] == strings.TrimSpace(t) {
				val, err := lookupRuntime(m[1], msgs)
				if err != nil && firstErr == nil {
					firstErr = err
				}
				return val
			}
			s, err := resolveRuntime(t, msgs)
			if err != nil && firstErr == nil {
				firstErr = err
			}
			return s
		case map[string]any:
			m := map[string]any{}
			for k, x := range t {
				m[k] = walk(x)
			}
			return m
		case []any:
			l := make([]any, len(t))
			for i, x := range t {
				l[i] = walk(x)
			}
			return l
		default:
			return v
		}
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = walk(in[k])
	}
	return out, firstErr
}

func lookupRuntime(expr string, msgs []Message) (any, error) {
	parts := strings.SplitN(expr, ".", 3)
	var content string
	var path string
	switch parts[0] {
	case "last":
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "tool" {
				content, _ = msgs[i].Content.(string)
				break
			}
		}
		if len(parts) > 1 {
			path = strings.Join(parts[1:], ".")
		}
	case "calls":
		if len(parts) < 2 {
			return nil, fmt.Errorf("bad template %s", expr)
		}
		prefix := "[call " + parts[1] + "]"
		for _, m := range msgs {
			if s, ok := m.Content.(string); ok && m.Role == "tool" && strings.HasPrefix(s, prefix) {
				content = s
			}
		}
		if len(parts) > 2 {
			path = parts[2]
		}
	}
	if content == "" {
		return nil, fmt.Errorf("template %s: no such tool result", expr)
	}
	body := stripToolFrame(content)
	if path == "" {
		return body, nil
	}
	// UseNumber keeps numbers as written: a nanosecond timestamp does not
	// survive a float64.
	var v any
	dec := json.NewDecoder(strings.NewReader(firstJSON(body)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("template %s: result is not JSON", expr)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("template %s: result is not JSON", expr)
	}
	for _, seg := range splitPath(path) {
		switch t := v.(type) {
		case map[string]any:
			v = t[seg]
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return nil, fmt.Errorf("template %s: bad index %s", expr, seg)
			}
			v = t[i]
		default:
			return nil, fmt.Errorf("template %s: cannot index %T with %s", expr, v, seg)
		}
	}
	if v == nil {
		return nil, fmt.Errorf("template %s resolved to nothing", expr)
	}
	return v, nil
}

func splitPath(p string) []string {
	p = strings.NewReplacer("[", ".", "]", "").Replace(p)
	var out []string
	for _, s := range strings.Split(p, ".") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stripToolFrame(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 && strings.HasPrefix(s, "[call ") {
		s = s[i+1:]
	}
	if i := strings.LastIndex(s, "\n[budget:"); i >= 0 {
		s = s[:i]
	}
	return s
}

func firstJSON(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "{["); i > 0 {
		s = s[i:]
	}
	if i := strings.Index(s, "\n[exit status"); i >= 0 {
		s = s[:i]
	}
	return s
}
