package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

type SDKConfigRuleInput struct {
	Name              string         `json:"name"`
	Enabled           *bool          `json:"enabled,omitempty"`
	Conditions        map[string]any `json:"conditions"`
	Settings          map[string]any `json:"settings"`
	ExpectedUpdatedAt string         `json:"expectedUpdatedAt,omitempty"`
}

type SDKConfigPreviewInput struct {
	Attributes  map[string]string `json:"attributes"`
	InstanceUID string            `json:"instanceUid,omitempty"`
}

func (c *Client) ListSDKConfigRules(ctx context.Context, projectID string) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodGet, "/api/sdk-config/rules"+projectQuery(projectID), nil, &resp)
	return resp, err
}

func (c *Client) CreateSDKConfigRule(ctx context.Context, projectID string, in SDKConfigRuleInput) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/sdk-config/rules"+projectQuery(projectID), in, &resp)
	return resp, err
}

func (c *Client) UpdateSDKConfigRule(ctx context.Context, projectID string, id int, in SDKConfigRuleInput) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPut, "/api/sdk-config/rules/"+strconv.Itoa(id)+projectQuery(projectID), in, &resp)
	return resp, err
}

func (c *Client) DeleteSDKConfigRule(ctx context.Context, projectID string, id int) error {
	return c.do(ctx, http.MethodDelete, "/api/sdk-config/rules/"+strconv.Itoa(id)+projectQuery(projectID), nil, nil)
}

func (c *Client) ReorderSDKConfigRules(ctx context.Context, projectID string, ids []int) error {
	return c.do(ctx, http.MethodPut, "/api/sdk-config/rules/reorder"+projectQuery(projectID), struct {
		IDs []int `json:"ids"`
	}{ids}, nil)
}

func (c *Client) PreviewSDKConfig(ctx context.Context, projectID string, in SDKConfigPreviewInput) (json.RawMessage, error) {
	var resp json.RawMessage
	err := c.do(ctx, http.MethodPost, "/api/sdk-config/preview"+projectQuery(projectID), in, &resp)
	return resp, err
}
