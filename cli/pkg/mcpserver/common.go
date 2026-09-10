package mcpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/cli/internal/errclass"
	"github.com/tracewayapp/traceway/cli/internal/timerange"
	"github.com/tracewayapp/traceway/cli/pkg/access"
	"github.com/tracewayapp/traceway/cli/pkg/access/traceway"
	"github.com/tracewayapp/traceway/cli/pkg/client"
)

// Tool errors carry the same stable snake_case codes as the CLI's error
// envelope, formatted as "error=<code>: <message>. Hint: <hint>" so MCP
// clients can branch on the code.

// usageErrf builds an invalid_argument tool error. The message should name
// the offending param and list the valid values inline.
func usageErrf(format string, args ...any) error {
	return fmt.Errorf("error=invalid_argument: "+format, args...)
}

// apiErr maps a pkg/client error via errclass, swapping the CLI's re-login
// hint for one that matches how this server's credentials were supplied (a
// stdio server has no TTY to log in with).
func (s *server) apiErr(err error) error {
	c := errclass.Classify(err)
	hint := c.Hint
	if c.Code == "token_expired" {
		hint = s.cfg.AuthHint
		if hint == "" {
			hint = "run 'traceway login' in a terminal on the host, then reconnect this MCP server"
		}
	}
	if hint != "" {
		return fmt.Errorf("error=%s: %s. Hint: %s", c.Code, c.Message, hint)
	}
	return fmt.Errorf("error=%s: %s", c.Code, c.Message)
}

func (s *server) client(req *mcp.CallToolRequest) *client.Client {
	if !s.cfg.PerRequestBearer || req == nil {
		return s.cfg.Client
	}
	extra := req.GetExtra()
	if extra == nil || extra.Header == nil {
		return s.cfg.Client
	}
	fields := strings.Fields(extra.Header.Get("Authorization"))
	if len(fields) != 2 || !strings.EqualFold(fields[0], "bearer") {
		return s.cfg.Client
	}
	return s.cfg.Client.WithBearer(fields[1])
}

// sources is the resolver for one call. A per-request bearer means the
// backend mount, where the only source is the instance itself under that
// bearer; otherwise it is the profile's full source list built at New.
func (s *server) sources(req *mcp.CallToolRequest) *access.Resolver {
	if c := s.client(req); c != s.cfg.Client {
		return access.NewResolver(access.Bound{Source: traceway.New(traceway.DefaultName, c)})
	}
	return s.resolver
}

// withSourceWarnings shapes a fan-out answer. Sources that failed while
// others answered are reported in a second text block so the structured
// result keeps its shape.
func withSourceWarnings(out any, failed []*access.SourceError) (*mcp.CallToolResult, any, error) {
	if len(failed) == 0 {
		return nil, out, nil
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return nil, nil, err
	}
	var warning strings.Builder
	for _, f := range failed {
		warning.WriteString("warning: ")
		warning.WriteString(f.Error())
		warning.WriteByte('\n')
	}
	return &mcp.CallToolResult{Content: []mcp.Content{
		&mcp.TextContent{Text: string(encoded)},
		&mcp.TextContent{Text: strings.TrimRight(warning.String(), "\n")},
	}}, out, nil
}

// project resolves the effective project id: the per-call param wins, then
// the session default.
func (s *server) project(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if s.cfg.DefaultProjectID != "" {
		return s.cfg.DefaultProjectID, nil
	}
	return "", errors.New("error=no_project: no project selected. Hint: pass project_id (find ids with list_projects) or set a default with 'traceway projects use <id>' on the host")
}

// projectIn is embedded in every project-scoped tool input.
type projectIn struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"Project to query. Optional when a default project is configured on the host (traceway projects use); find ids with list_projects."`
}

// sourceIn is embedded in every telemetry tool input.
type sourceIn struct {
	Source string `json:"source,omitempty" jsonschema:"Telemetry source to query by name when the profile binds more than one (traceway sources list). Default: every source that answers the domain, merged, each record tagged with its source."`
}

// timeRangeIn is embedded in every windowed list/query tool input.
type timeRangeIn struct {
	Since string `json:"since,omitempty" jsonschema:"Relative window ending now, like 30m, 1h, 24h, 7d. Units s/m/h and integer d only: no 1w, no 7d2h. Default 1h. Mutually exclusive with from/to."`
	From  string `json:"from,omitempty" jsonschema:"Window start, RFC3339 like 2026-06-23T14:00:00Z. Requires to."`
	To    string `json:"to,omitempty" jsonschema:"Window end, RFC3339. Requires from."`
}

func (t timeRangeIn) resolve() (client.TimeRange, error) {
	tr, err := timerange.Resolve(t.Since, t.From, t.To)
	if err != nil {
		return client.TimeRange{}, usageErrf("%v. Use since like 30m, 1h, 7d (units s/m/h and integer d only), or from+to as RFC3339; not both", err)
	}
	return tr, nil
}

// pageIn is embedded in every paginated tool input. The default page size is
// 20, below the CLI's 50: tool results land in model context.
type pageIn struct {
	Page     int `json:"page,omitempty" jsonschema:"Page number, 1-indexed. Default 1."`
	PageSize int `json:"page_size,omitempty" jsonschema:"Records per page, 1 to 100. Default 20; keep it at 10 to 20 for triage."`
}

func (p pageIn) resolve() (client.PaginationParams, error) {
	page, size := p.Page, p.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page < 1 {
		return client.PaginationParams{}, usageErrf("page must be 1 or greater")
	}
	if size < 1 || size > 100 {
		return client.PaginationParams{}, usageErrf("page_size must be between 1 and 100")
	}
	return client.PaginationParams{Page: page, PageSize: size}, nil
}

// parseTimestamp parses a required RFC3339 timestamp param. By-id lookups
// must bound their query so ClickHouse prunes daily partitions.
func parseTimestamp(param, value string) (time.Time, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, usageErrf("%s is required: pass the record's timestamp (RFC3339, approximate within a day is fine). Recover it from the dashboard URL's ?t= param, an occurrence's recordedAt, or a notification's Occurred at", param)
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, usageErrf("%s: %v. Pass RFC3339 like 2026-06-23T14:30:00Z", param, err)
	}
	return t, nil
}

// validateEnum rejects a non-empty value outside allowed. Empty means unset
// and is left to the caller's default.
func validateEnum(param, value string, allowed []string) error {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}
	return usageErrf("%s must be one of: %s", param, strings.Join(allowed, ", "))
}

// validateUUID mirrors the CLI's validateUUIDArg for by-id tool params.
func validateUUID(param, value string) error {
	if _, err := uuid.Parse(value); err != nil {
		return usageErrf("invalid %s %q: must be a UUID", param, value)
	}
	return nil
}

var exceptionHashPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true}
}

// mutating marks the archive tools: they write, but reversibly (archive and
// unarchive undo each other), and repeating a call changes nothing.
func mutating() *mcp.ToolAnnotations {
	nonDestructive := false
	return &mcp.ToolAnnotations{DestructiveHint: &nonDestructive, IdempotentHint: true}
}
