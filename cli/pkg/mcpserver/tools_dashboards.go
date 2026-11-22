package mcpserver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func (s *server) addDashboardReadTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "list_dashboards", Description: "List dashboards applied to the selected project.", OutputSchema: passThroughSchema, Annotations: readOnly()}, s.listDashboards)
	mcp.AddTool(srv, &mcp.Tool{Name: "dashboard_library", Description: "List dashboards across accessible organizations with appliedProjectIds. Inspect shared scope before upsert.", OutputSchema: passThroughSchema, Annotations: readOnly()}, s.dashboardLibrary)
	mcp.AddTool(srv, &mcp.Tool{Name: "export_dashboard", Description: "Export a dashboard as importable JSON. Preserve widget IDs and unrelated widgets when editing.", OutputSchema: passThroughSchema, Annotations: readOnly()}, s.exportDashboard)
	mcp.AddTool(srv, &mcp.Tool{Name: "discover_metrics", Description: "Discover recorded metric names, types, units and tag keys. Supply both name and key to discover exact tag values. Empty results mean no observations in this project/time range.", OutputSchema: passThroughSchema, Annotations: readOnly()}, s.discoverMetrics)
}

func (s *server) addDashboardWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "create_dashboard_embed", Description: "Issue an expiring read-only link to a dashboard applied to the selected project. Anyone holding the link views the dashboard without a Traceway account, directly or in an iframe, limited to the signed time range and tag_filters. The result carries the signed token (also inside url and embedPath): treat it as a credential and hand it only to the user who asked. It expires after five minutes unless an expiry is given (90 days at most) and cannot be revoked on its own; removing the dashboard from the project stops every link for that project. Only issue when requested by the user. To embed dashboards in the user's own application, their backend calls the same API per authorized viewer: read traceway://knowledge/dashboard-embedding.", OutputSchema: passThroughSchema, Annotations: additive()}, s.createDashboardEmbed)
	mcp.AddTool(srv, &mcp.Tool{Name: "import_dashboards", Description: "Create dashboards from a single JSON document or bundle and apply to the selected project. mode create (default) creates copies, even on retries; mode upsert matches names case-insensitively within the organization and replaces entire definitions/descriptions in ALL assigned projects, without concurrent-edit protection. Inspect dashboard_library and export_dashboard first. Only write when requested by the user. Read traceway://knowledge/dashboard-building.", OutputSchema: passThroughSchema, Annotations: destructive()}, s.importDashboards)
}

type dashboardIDIn struct {
	projectIn
	ID int `json:"id" jsonschema:"Dashboard numeric ID from list_dashboards or dashboard_library."`
}
type discoverMetricsIn struct {
	projectIn
	timeRangeIn
	Name string `json:"name,omitempty" jsonschema:"Metric name for tag-value discovery; requires key."`
	Key  string `json:"key,omitempty" jsonschema:"Tag key for tag-value discovery; requires name."`
}
type importDashboardsIn struct {
	projectIn
	Data map[string]any `json:"data" jsonschema:"Dashboard document with schemaVersion 1, name, optional description and widgets; or bundle with schemaVersion 1 and dashboards array. Widgets have id, title, widgetType and config.sources (type metric, name, aggregation, optional tagFilters/groupBy/label/complement)."`
	Mode string         `json:"mode,omitempty" jsonschema:"create (default) or upsert. Upsert replaces organization-wide matching names."`
}

type dashboardEmbedIn struct {
	projectIn
	ID            int               `json:"id" jsonschema:"Dashboard numeric ID from list_dashboards. The dashboard must be applied to the project."`
	Timeframe     string            `json:"timeframe,omitempty" jsonschema:"Relative window: a positive whole number followed by m, h or d, such as 30m, 42h or 7d, up to 731 days. Default the last 42 hours. With an expiry it is resolved each time the link is opened; without one, at issuance. Excludes from and to."`
	From          string            `json:"from,omitempty" jsonschema:"Start of a fixed range, RFC 3339. Requires to."`
	To            string            `json:"to,omitempty" jsonschema:"End of a fixed range, RFC 3339. Requires from."`
	TagFilters    map[string]string `json:"tag_filters,omitempty" jsonschema:"Tag values every widget is limited to, such as {\"account_id\":\"acme\"}. They win over a filter saved on a widget for the same tag. At most 20, no empty tag or value."`
	ExpiresInDays int               `json:"expires_in_days,omitempty" jsonschema:"Lifetime in whole days, 1 to 90. Default five minutes. Excludes expires_at."`
	ExpiresAt     string            `json:"expires_at,omitempty" jsonschema:"Expiry as an RFC 3339 timestamp, at most 90 days ahead."`
	Theme         string            `json:"theme,omitempty" jsonschema:"light or dark (default dark)."`
}

func (s *server) createDashboardEmbed(ctx context.Context, req *mcp.CallToolRequest, in dashboardEmbedIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	opts := client.DashboardEmbedOptions{Timeframe: in.Timeframe, TagFilters: in.TagFilters, ExpiresInDays: in.ExpiresInDays, Theme: in.Theme}
	for param, target := range map[string]struct {
		raw string
		at  *time.Time
	}{"from": {in.From, &opts.From}, "to": {in.To, &opts.To}, "expires_at": {in.ExpiresAt, &opts.ExpiresAt}} {
		if target.raw == "" {
			continue
		}
		if *target.at, err = time.Parse(time.RFC3339, target.raw); err != nil {
			return nil, nil, usageErrf("%s must be an RFC 3339 timestamp such as 2026-06-23T14:30:00Z", param)
		}
	}
	if err := opts.Validate(); err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	c := s.client(req)
	origin := s.cfg.InstanceURL
	if origin == "" {
		origin = c.BaseURL
	}
	return s.raw(c.CreateDashboardEmbed(ctx, id, in.ID, opts, origin))
}

func (s *server) listDashboards(ctx context.Context, req *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ListDashboards(ctx, id))
}
func (s *server) dashboardLibrary(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
	return s.raw(s.client(req).DashboardLibrary(ctx))
}
func (s *server) exportDashboard(ctx context.Context, req *mcp.CallToolRequest, in dashboardIDIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ExportDashboard(ctx, id, in.ID))
}
func (s *server) discoverMetrics(ctx context.Context, req *mcp.CallToolRequest, in discoverMetricsIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if (in.Name == "") != (in.Key == "") {
		return nil, nil, usageErrf("supply both name and key for tag-value discovery")
	}
	tr, err := in.timeRangeIn.resolve()
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).DiscoverMetrics(ctx, id, tr, in.Name, in.Key))
}
func (s *server) importDashboards(ctx context.Context, req *mcp.CallToolRequest, in importDashboardsIn) (*mcp.CallToolResult, any, error) {
	id, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if in.Mode == "" {
		in.Mode = "create"
	}
	data, err := json.Marshal(in.Data)
	if err != nil {
		return nil, nil, usageErrf("invalid dashboard data: %v", err)
	}
	if err := client.ValidateDashboardImport(data, in.Mode); err != nil {
		return nil, nil, usageErrf("%v", err)
	}
	return s.raw(s.client(req).ImportDashboards(ctx, id, data, in.Mode))
}
