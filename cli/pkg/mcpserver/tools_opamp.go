package mcpserver

import (
	"context"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tracewayapp/traceway/cli/v2/pkg/client"
)

func (s *server) addOpampReadTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "list_sdk_config_rules", OutputSchema: passThroughSchema,
		Description: "List ordered OpAMP browser SDK rules, including conditions, settings and updatedAt. Later matching rules win. Browser frontend projects only; inspect before changing collection.", Annotations: readOnly()}, s.listSDKConfigRules)
	mcp.AddTool(srv, &mcp.Tool{Name: "preview_sdk_config", OutputSchema: passThroughSchema,
		Description: "Evaluate saved OpAMP rules for client attributes and optional instance UID. Returns effective settings, matched rules, plan limits, config hash and heartbeat interval. Read-only; this does not prove a browser has applied the config.", Annotations: readOnly()}, s.previewSDKConfig)
}

func (s *server) addOpampWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{Name: "create_sdk_config_rule", OutputSchema: passThroughSchema,
		Description: "Create an OpAMP collection rule, appended after existing rules. Empty conditions targets every browser; later matching rules win. Settings can stream sessions, breadcrumbs and replay or turn them off. MUTATING: only on the user's request; explain targeting, sampling and replay masking before saving. Requires agent writes enabled.", Annotations: additive()}, s.createSDKConfigRule)
	mcp.AddTool(srv, &mcp.Tool{Name: "update_sdk_config_rule", OutputSchema: passThroughSchema,
		Description: "Replace an OpAMP rule's name, conditions and settings; omitted enabled preserves its state. Read list_sdk_config_rules first, preserve unrelated settings and triggers, and pass its updatedAt as expected_updated_at. A stale edit fails with 409. MUTATING: affects matching browsers on the next poll.", Annotations: mutating()}, s.updateSDKConfigRule)
	mcp.AddTool(srv, &mcp.Tool{Name: "delete_sdk_config_rule", OutputSchema: passThroughSchema,
		Description: "Delete an OpAMP rule. Earlier rules and SDK defaults take over, which may increase or reduce collection. MUTATING: only when requested; prefer disabling via update when temporary.", Annotations: destructive()}, s.deleteSDKConfigRule)
	mcp.AddTool(srv, &mcp.Tool{Name: "reorder_sdk_config_rules", OutputSchema: passThroughSchema,
		Description: "Set OpAMP rule evaluation order. Include every project rule id exactly once; later matching rules win. MUTATING: inspect existing rules and explain precedence changes first.", Annotations: mutating()}, s.reorderSDKConfigRules)
}

type sdkRuleIn struct {
	projectIn
	Name       string         `json:"name" jsonschema:"Rule name, required."`
	Enabled    *bool          `json:"enabled,omitempty" jsonschema:"Defaults true on create; omitted on update preserves state."`
	Conditions map[string]any `json:"conditions" jsonschema:"Targeting document; {} matches every browser. Use all: [{key, op, value}] and rollout: {percent: 0..100, by: instance or an attribute key}."`
	Settings   map[string]any `json:"settings" jsonschema:"Partial SDK settings document (schemaVersion 1): enabled kill switch; signals modes off/buffer/stream and sampleRate 0..1; sessions enabled/baselineSampleRate; replay mode/sessionSampleRate/maskAllInputs; buffer; triggers. Server-owned captures and limits are forbidden."`
}

type updateSDKRuleIn struct {
	sdkRuleIn
	ID                int    `json:"id" jsonschema:"Rule id from list_sdk_config_rules."`
	ExpectedUpdatedAt string `json:"expected_updated_at" jsonschema:"Exact updatedAt from the rule read before editing; stale updates fail with 409."`
}

type sdkRuleIDIn struct {
	projectIn
	ID int `json:"id" jsonschema:"Rule id from list_sdk_config_rules."`
}

type reorderSDKRulesIn struct {
	projectIn
	IDs []int `json:"ids" jsonschema:"Every rule id exactly once, in evaluation order; later matching rules win."`
}

type previewSDKConfigIn struct {
	projectIn
	Attributes  map[string]string `json:"attributes" jsonschema:"Client attributes such as service.version, deployment.environment.name, user.id or session.id. {} previews an anonymous client."`
	InstanceUID string            `json:"instance_uid,omitempty" jsonschema:"Browser instance UUID, used for sticky rollout evaluation."`
}

func (in sdkRuleIn) input() (client.SDKConfigRuleInput, error) {
	if strings.TrimSpace(in.Name) == "" || in.Conditions == nil || in.Settings == nil {
		return client.SDKConfigRuleInput{}, usageErrf("name, conditions and settings are required; use {} conditions for all clients")
	}
	return client.SDKConfigRuleInput{Name: in.Name, Enabled: in.Enabled, Conditions: in.Conditions, Settings: in.Settings}, nil
}

func (s *server) listSDKConfigRules(ctx context.Context, req *mcp.CallToolRequest, in projectIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).ListSDKConfigRules(ctx, projectID))
}

func (s *server) previewSDKConfig(ctx context.Context, req *mcp.CallToolRequest, in previewSDKConfigIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).PreviewSDKConfig(ctx, projectID, client.SDKConfigPreviewInput{Attributes: in.Attributes, InstanceUID: in.InstanceUID}))
}

func (s *server) createSDKConfigRule(ctx context.Context, req *mcp.CallToolRequest, in sdkRuleIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	input, err := in.input()
	if err != nil {
		return nil, nil, err
	}
	return s.raw(s.client(req).CreateSDKConfigRule(ctx, projectID, input))
}

func (s *server) updateSDKConfigRule(ctx context.Context, req *mcp.CallToolRequest, in updateSDKRuleIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	input, err := in.input()
	if err != nil {
		return nil, nil, err
	}
	if _, err := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt); err != nil {
		return nil, nil, usageErrf("expected_updated_at must be the updatedAt timestamp from list_sdk_config_rules")
	}
	input.ExpectedUpdatedAt = in.ExpectedUpdatedAt
	return s.raw(s.client(req).UpdateSDKConfigRule(ctx, projectID, in.ID, input))
}

func (s *server) deleteSDKConfigRule(ctx context.Context, req *mcp.CallToolRequest, in sdkRuleIDIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	if err := positiveID("id", in.ID); err != nil {
		return nil, nil, err
	}
	if err := s.client(req).DeleteSDKConfigRule(ctx, projectID, in.ID); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, map[string]any{"status": "deleted", "id": in.ID}, nil
}

func (s *server) reorderSDKConfigRules(ctx context.Context, req *mcp.CallToolRequest, in reorderSDKRulesIn) (*mcp.CallToolResult, any, error) {
	projectID, err := s.project(in.ProjectID)
	if err != nil {
		return nil, nil, err
	}
	for _, id := range in.IDs {
		if err := positiveID("ids", id); err != nil {
			return nil, nil, err
		}
	}
	if err := s.client(req).ReorderSDKConfigRules(ctx, projectID, in.IDs); err != nil {
		return nil, nil, s.apiErr(err)
	}
	return nil, map[string]any{"status": "reordered", "ids": in.IDs}, nil
}
