// Package ops maps CLI invocations and MCP tool calls onto surface-neutral
// operations (endpoints.show, pages.ack, ...) so one case spec grades both.
package ops

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/tracewayapp/traceway/cli/pkg/mcpserver/knowledge"
)

// An operation is the surface-neutral name of what a tool call did
// (endpoints.show, pages.ack, read_file, ...), so one case spec grades CLI and
// MCP runs alike. Args are keyed by the MCP parameter name, which is the CLI
// flag in snake_case except where Spec.Rename says otherwise.

// Spec describes one Traceway operation on both surfaces.
type Spec struct {
	Op    string   // endpoints.show
	CLI   []string // command path after the binary: [endpoints show]
	MCP   string   // MCP tool name; empty for CLI-only commands
	Write bool
	// Positional names the CLI positional args in order; a trailing "..."
	// collects the rest into a list (exceptions archive <hash>...).
	Positional []string
	// Rename maps a snake_case CLI flag to its MCP parameter.
	Rename map[string]string
	// WriteEdition marks commands that exist only in traceway-write.
	WriteEdition bool
}

// specs is the static half of the operations map: positional names, renames
// and write flags. The CLI<->MCP pairing itself is parsed from tool-map.md and
// cross-checked against this table by TestOpsMatchToolMap.
var specs = []Spec{
	{Op: "projects.list", CLI: []string{"projects", "list"}, MCP: "list_projects"},
	{Op: "projects.use", CLI: []string{"projects", "use"}, Positional: []string{"project_id"}},
	{Op: "exceptions.list", CLI: []string{"exceptions", "list"}, MCP: "list_exceptions"},
	{Op: "exceptions.show", CLI: []string{"exceptions", "show"}, MCP: "get_exception", Positional: []string{"hash"}},
	{Op: "exceptions.occurrence", CLI: []string{"exceptions", "occurrence"}, MCP: "get_exception_occurrence", Positional: []string{"id"}},
	{Op: "logs.query", CLI: []string{"logs", "query"}, MCP: "query_logs", Rename: map[string]string{"service": "service_name"}},
	{Op: "endpoints.list", CLI: []string{"endpoints", "list"}, MCP: "list_endpoints"},
	{Op: "endpoints.requests", CLI: []string{"endpoints", "requests"}, MCP: "list_endpoint_requests", Positional: []string{"endpoint"}},
	{Op: "endpoints.show", CLI: []string{"endpoints", "show"}, MCP: "get_endpoint_request", Positional: []string{"id"}},
	{Op: "endpoints.chart", CLI: []string{"endpoints", "chart"}, MCP: "endpoints_chart"},
	{Op: "endpoints.slow", CLI: []string{"endpoints", "slow"}, MCP: "get_slow_endpoint_config", Positional: []string{"endpoint"}},
	{Op: "tasks.show", CLI: []string{"tasks", "show"}, MCP: "get_task", Positional: []string{"id"}},
	{Op: "ai_traces.show", CLI: []string{"ai-traces", "show"}, MCP: "get_ai_trace", Positional: []string{"id"}},
	{Op: "sessions.show", CLI: []string{"sessions", "show"}, MCP: "get_session", Positional: []string{"id"}},
	{Op: "traces.show", CLI: []string{"traces", "show"}, MCP: "get_trace", Positional: []string{"id"}},
	{Op: "metrics.query", CLI: []string{"metrics", "query"}, MCP: "query_metrics", Rename: map[string]string{"tag": "tag_filters"}},
	{Op: "pages.list", CLI: []string{"pages", "list"}, MCP: "list_pages"},
	{Op: "pages.show", CLI: []string{"pages", "show"}, MCP: "get_page", Positional: []string{"id"}},
	{Op: "alerts.list", CLI: []string{"alerts", "list"}, MCP: "list_notification_rules"},
	{Op: "monitors.list", CLI: []string{"monitors", "list"}, MCP: "list_checks"},
	{Op: "monitors.show", CLI: []string{"monitors", "show"}, MCP: "get_check", Positional: []string{"id"}},
	{Op: "post_mortems.list", CLI: []string{"post-mortems", "list"}, MCP: "list_post_mortems", Rename: map[string]string{"tag": "tags"}},
	{Op: "post_mortems.show", CLI: []string{"post-mortems", "show"}, MCP: "get_post_mortem", Positional: []string{"id"}},

	{Op: "exceptions.archive", CLI: []string{"exceptions", "archive"}, MCP: "archive_exceptions", Write: true, WriteEdition: true, Positional: []string{"hashes..."}},
	{Op: "exceptions.unarchive", CLI: []string{"exceptions", "unarchive"}, MCP: "unarchive_exceptions", Write: true, WriteEdition: true, Positional: []string{"hashes..."}},
	{Op: "pages.ack", CLI: []string{"pages", "ack"}, MCP: "acknowledge_page", Write: true, WriteEdition: true, Positional: []string{"id"}},
	{Op: "pages.resolve", CLI: []string{"pages", "resolve"}, MCP: "resolve_page", Write: true, WriteEdition: true, Positional: []string{"id"}},
	{Op: "alerts.enable", CLI: []string{"alerts", "enable"}, MCP: "set_notification_rule_enabled", Write: true, WriteEdition: true, Positional: []string{"id"}},
	{Op: "alerts.disable", CLI: []string{"alerts", "disable"}, MCP: "set_notification_rule_enabled", Write: true, WriteEdition: true, Positional: []string{"id"}},
	{Op: "alerts.snooze", CLI: []string{"alerts", "snooze"}, MCP: "snooze_notification_rule", Write: true, WriteEdition: true, Positional: []string{"id"}, Rename: map[string]string{"minutes": "duration_minutes"}},
	{Op: "monitors.run", CLI: []string{"monitors", "run"}, MCP: "run_check", Write: true, WriteEdition: true, Positional: []string{"id"}},
	{Op: "post_mortems.create", CLI: []string{"post-mortems", "create"}, MCP: "create_post_mortem", Write: true, WriteEdition: true, Rename: map[string]string{"tag": "tags", "incident": "incident_id", "content_file": "content_md"}},
	{Op: "post_mortems.update", CLI: []string{"post-mortems", "update"}, MCP: "update_post_mortem", Write: true, WriteEdition: true, Positional: []string{"id"}, Rename: map[string]string{"tag": "tags", "incident": "incident_id", "content_file": "content_md"}},
	{Op: "endpoints.mark_slow", CLI: []string{"endpoints", "mark-slow"}, MCP: "set_slow_endpoint", Write: true, WriteEdition: true, Positional: []string{"endpoint"}},

	// CLI-only commands. setup apply proposes a plan draft; nothing is
	// created until a person approves it, so it is not an agent write.
	{Op: "setup.apply", CLI: []string{"setup", "apply"}},
	{Op: "login", CLI: []string{"login"}},
	{Op: "logout", CLI: []string{"logout"}},
	{Op: "profiles.list", CLI: []string{"profiles", "list"}},
	{Op: "profiles.use", CLI: []string{"profiles", "use"}, Positional: []string{"profile"}},
	{Op: "version", CLI: []string{"version"}},
	{Op: "mcp.serve", CLI: []string{"mcp"}},
}

// Harness tool operations that are not Traceway commands.
const (
	OpReadFile     = "read_file"
	OpListFiles    = "list_files"
	OpSearch       = "search"
	OpEditFile     = "edit_file"
	OpWriteFile    = "write_file"
	OpLoadSkill    = "load_skill"
	OpAskUser      = "ask_user"
	OpReadResource = "read_resource"
	OpListRes      = "list_resources"
	OpShell        = "shell"
	OpGit          = "git"
	OpSubmit       = "submit"
	OpUnknownCLI   = "traceway.unknown"
	// OpHelp is any traceway invocation with --help: it reads the usage, not
	// the data, so it must not satisfy a matcher for the command itself.
	OpHelp = "traceway.help"
)

// boolFlags lists CLI flags that take no value.
var boolFlags = map[string]bool{
	"yes": true, "include_archived": true, "archive_issue": true, "unlink_incident": true,
	"password": true, "password_stdin": true, "token_stdin": true, "no_browser": true,
	"help": true, "version": true,
}

var shortFlags = map[string]string{"o": "output", "h": "help", "v": "version"}

var (
	opsOnce   sync.Once
	byOp      map[string]*Spec
	byMCP     map[string][]*Spec
	cliSorted []*Spec
)

func initOps() {
	opsOnce.Do(func() {
		byOp = map[string]*Spec{}
		byMCP = map[string][]*Spec{}
		for i := range specs {
			s := &specs[i]
			byOp[s.Op] = s
			if s.MCP != "" {
				byMCP[s.MCP] = append(byMCP[s.MCP], s)
			}
			cliSorted = append(cliSorted, s)
		}
		sort.SliceStable(cliSorted, func(i, j int) bool { return len(cliSorted[i].CLI) > len(cliSorted[j].CLI) })
	})
}

// Specs returns the operations table.
func Specs() []Spec { return specs }

// SpecFor returns an operation's spec.
func SpecFor(op string) (*Spec, bool) {
	initOps()
	s, ok := byOp[op]
	return s, ok
}

// IsWriteOp reports whether an op mutates Traceway state.
func IsWriteOp(op string) bool {
	s, ok := SpecFor(op)
	return ok && s.Write
}

// Op is one recorded operation.
type Op struct {
	Op    string         `json:"op"`
	Args  map[string]any `json:"args,omitempty"`
	Write bool           `json:"write,omitempty"`
}

// ParseCLI maps a traceway[-write] argv (without the binary) to an op.
func ParseCLI(argv []string) Op {
	initOps()
	var words []string
	flags := map[string]any{}
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			words = append(words, argv[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") && len(a) > 1 {
			name, val, hasVal := strings.Cut(strings.TrimLeft(a, "-"), "=")
			if !strings.HasPrefix(a, "--") {
				if long, ok := shortFlags[name]; ok {
					name = long
				}
			}
			key := strings.ReplaceAll(name, "-", "_")
			if !hasVal && !boolFlags[key] && i+1 < len(argv) {
				val = argv[i+1]
				i++
			}
			if !hasVal && boolFlags[key] {
				addArg(flags, key, true)
				continue
			}
			addArg(flags, key, val)
			continue
		}
		words = append(words, a)
	}
	if flags["help"] != nil {
		return Op{Op: OpHelp, Args: map[string]any{"command": strings.Join(words, " ")}}
	}
	var spec *Spec
	for _, s := range cliSorted {
		if len(words) >= len(s.CLI) && equalWords(words[:len(s.CLI)], s.CLI) {
			spec = s
			break
		}
	}
	if spec == nil {
		return Op{Op: OpUnknownCLI, Args: map[string]any{"argv": strings.Join(argv, " ")}}
	}
	args := map[string]any{}
	for k, v := range flags {
		if r, ok := spec.Rename[k]; ok {
			k = r
		}
		if k == "project" {
			k = "project_id"
		}
		args[k] = v
	}
	pos := words[len(spec.CLI):]
	for i, name := range spec.Positional {
		if strings.HasSuffix(name, "...") {
			if i < len(pos) {
				list := make([]any, 0, len(pos)-i)
				for _, p := range pos[i:] {
					list = append(list, p)
				}
				args[strings.TrimSuffix(name, "...")] = list
			}
			break
		}
		if i < len(pos) {
			args[name] = pos[i]
		}
	}
	return Op{Op: spec.Op, Args: args, Write: spec.Write}
}

func addArg(m map[string]any, k string, v any) {
	prev, ok := m[k]
	if !ok {
		m[k] = v
		return
	}
	switch p := prev.(type) {
	case []any:
		m[k] = append(p, v)
	default:
		m[k] = []any{p, v}
	}
}

func equalWords(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ToolMapPair is one row of tool-map.md.
type ToolMapPair struct {
	CLI []string
	MCP string
}

var (
	toolMapRowRe = regexp.MustCompile("^\\|\\s*(`[^|]+`(?:\\s*/\\s*`[^|]+`)*)\\s*\\|\\s*(`[^|]+`(?:\\s*/\\s*`[^|]+`)*)\\s*\\|")
	codeRe       = regexp.MustCompile("`([^`]+)`")
)

// ParseToolMap reads the CLI->MCP pairs from tool-map.md. A row like
// "`pages list` / `pages show <id>` | `list_pages` / `get_page`" yields two
// pairs. Commands that elide the binary and noun ("`unarchive`") inherit them
// from the previous command in the cell.
func ParseToolMap() []ToolMapPair {
	var out []ToolMapPair
	for _, line := range strings.Split(knowledge.MustRead("tool-map.md"), "\n") {
		m := toolMapRowRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		clis := codeRe.FindAllStringSubmatch(m[1], -1)
		tools := codeRe.FindAllStringSubmatch(m[2], -1)
		var prev []string
		var cmds [][]string
		for _, c := range clis {
			words := commandWords(c[1])
			if len(words) == 0 {
				continue
			}
			if len(words) == 1 && len(prev) >= 1 {
				words = append(append([]string{}, prev[:len(prev)-1]...), words[0])
			} else if len(words) >= 1 && words[0] != prev0(prev) && len(prev) > 0 && !isNoun(words[0]) {
				words = append([]string{prev[0]}, words...)
			}
			prev = words
			cmds = append(cmds, words)
		}
		for i, c := range cmds {
			t := tools[len(tools)-1][1]
			if i < len(tools) {
				t = tools[i][1]
			}
			out = append(out, ToolMapPair{CLI: c, MCP: t})
		}
	}
	return out
}

func prev0(p []string) string {
	if len(p) == 0 {
		return ""
	}
	return p[0]
}

var nouns = map[string]bool{
	"projects": true, "exceptions": true, "logs": true, "endpoints": true, "tasks": true, "ai-traces": true,
	"sessions": true, "traces": true, "metrics": true, "pages": true, "alerts": true, "monitors": true, "post-mortems": true,
}

func isNoun(w string) bool { return nouns[w] }

// commandWords strips the binary name, flags and placeholders from a code span.
func commandWords(s string) []string {
	var out []string
	for _, w := range strings.Fields(s) {
		if w == "traceway" || w == "traceway-write" {
			continue
		}
		if strings.HasPrefix(w, "-") || strings.HasPrefix(w, "<") || w == "N" {
			break
		}
		out = append(out, w)
	}
	return out
}

// RenderCLI renders an op back into a traceway argv (used by the fake model
// to play surface-neutral oracle steps on CLI arms).
func RenderCLI(op string, args map[string]any) ([]string, error) {
	spec, ok := SpecFor(op)
	if !ok {
		return nil, fmt.Errorf("unknown op %q", op)
	}
	argv := append([]string{}, spec.CLI...)
	used := map[string]bool{}
	for _, name := range spec.Positional {
		key := strings.TrimSuffix(name, "...")
		v, ok := args[key]
		if !ok {
			continue
		}
		used[key] = true
		if list, ok := v.([]any); ok {
			for _, x := range list {
				argv = append(argv, fmt.Sprint(x))
			}
		} else {
			argv = append(argv, fmt.Sprint(v))
		}
	}
	reverse := map[string]string{}
	for cli, mcp := range spec.Rename {
		reverse[mcp] = cli
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if used[k] {
			continue
		}
		v := args[k]
		flag := k
		if c, ok := reverse[k]; ok {
			flag = c
		}
		if k == "project_id" {
			flag = "project"
		}
		flag = "--" + strings.ReplaceAll(flag, "_", "-")
		switch t := v.(type) {
		case bool:
			if t {
				argv = append(argv, flag)
			}
		case []any:
			for _, x := range t {
				argv = append(argv, flag, fmt.Sprint(x))
			}
		case map[string]any:
			mk := make([]string, 0, len(t))
			for kk := range t {
				mk = append(mk, kk)
			}
			sort.Strings(mk)
			for _, kk := range mk {
				argv = append(argv, flag, kk+"="+fmt.Sprint(t[kk]))
			}
		default:
			argv = append(argv, flag, fmt.Sprint(v))
		}
	}
	return argv, nil
}
