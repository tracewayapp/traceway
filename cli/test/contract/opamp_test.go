package contract

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/tracewayapp/traceway/cli/pkg/client"
)

func TestContractOpampRules(t *testing.T) {
	status, _, project := doJSON(t, http.MethodPost, "/api/projects?projectId="+projectID, jwtToken, map[string]any{
		"name": "OpAMP contract browser", "framework": "svelte", "organizationId": organizationOfProject(t),
	})
	if status != http.StatusCreated {
		t.Fatalf("create browser project: %d %v", status, project)
	}
	id, ok := project["id"].(string)
	if !ok {
		t.Fatalf("missing project id: %v", project)
	}
	setAgentWrites(t, true)
	t.Cleanup(func() { setAgentWrites(t, false) })
	c := client.New(baseURL, client.WithJWT(oauthScopedAccessToken(t, "traceway:write")))
	ctx := t.Context()
	input := client.SDKConfigRuleInput{
		Name: "Full sessions", Conditions: map[string]any{},
		Settings: map[string]any{
			"sessions": map[string]any{"enabled": true, "baselineSampleRate": 1},
			"replay":   map[string]any{"mode": "stream", "sessionSampleRate": 1, "maskAllInputs": true},
		},
	}
	raw, err := c.CreateSDKConfigRule(ctx, id, input)
	if err != nil {
		t.Fatal(err)
	}
	var rule struct {
		ID        int    `json:"id"`
		UpdatedAt string `json:"updatedAt"`
	}
	if err := json.Unmarshal(raw, &rule); err != nil || rule.ID == 0 || rule.UpdatedAt == "" {
		t.Fatalf("create response: %s (%v)", raw, err)
	}
	if _, err := c.ListSDKConfigRules(ctx, id); err != nil {
		t.Fatal(err)
	}
	preview := func(want string) {
		t.Helper()
		raw, err := c.PreviewSDKConfig(ctx, id, client.SDKConfigPreviewInput{Attributes: map[string]string{}})
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Settings struct {
				Replay struct {
					Mode string `json:"mode"`
				} `json:"replay"`
			} `json:"settings"`
		}
		if err := json.Unmarshal(raw, &result); err != nil || result.Settings.Replay.Mode != want {
			t.Fatalf("preview: %s (%v), want %s", raw, err, want)
		}
	}
	preview("stream")
	input.ExpectedUpdatedAt = rule.UpdatedAt
	input.Settings["replay"] = map[string]any{"mode": "buffer", "maskAllInputs": true}
	if _, err := c.UpdateSDKConfigRule(ctx, id, rule.ID, input); err != nil {
		t.Fatal(err)
	}
	var conflict *client.APIError
	if _, err := c.UpdateSDKConfigRule(ctx, id, rule.ID, input); !errors.As(err, &conflict) || conflict.StatusCode != http.StatusConflict {
		t.Fatalf("stale update: %v", err)
	}
	preview("buffer")
	if err := c.ReorderSDKConfigRules(ctx, id, []int{rule.ID}); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSDKConfigRule(ctx, id, rule.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeleteSDKConfigRule(ctx, id, rule.ID); !errors.Is(err, client.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}
