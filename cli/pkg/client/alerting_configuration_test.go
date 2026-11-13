package client

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAlertingConfigurationRouting(t *testing.T) {
	var got string
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.Path
		if r.URL.Query().Get("projectId") != "project" {
			t.Error("missing project authorization context")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	for _, op := range AlertingOperations() {
		req := AlertingRequest{ProjectID: "project", OrganizationID: "7", ID: 3, ChildID: 4, From: "2026-01-01T00:00:00Z", To: "2026-01-02T00:00:00Z"}
		if op.Body {
			req.Body = alertingTestBody(op.Resource, op.Action)
		}
		_, err := c.ConfigureAlerting(context.Background(), op.Resource, op.Action, req)
		if err != nil {
			t.Fatalf("%s %s: %v", op.Resource, op.Action, err)
		}
		want := op.Method + " " + strings.NewReplacer("{organizationId}", "7", "{id}", "3", "{childId}", "4").Replace(op.Path)
		if got != want {
			t.Errorf("got %s want %s", got, want)
		}
	}
}
func alertingTestBody(resource, action string) json.RawMessage {
	if action == "set-members" {
		return json.RawMessage(`{"userIds":[]}`)
	}
	if action == "set-projects" {
		return json.RawMessage(`{"projectIds":[]}`)
	}
	if action == "create-override" {
		return json.RawMessage(`{"userId":1,"startAt":"2026-01-01T00:00:00Z","endAt":"2026-01-02T00:00:00Z"}`)
	}
	if action == "verify" {
		return json.RawMessage(`{"code":"123456"}`)
	}
	return json.RawMessage(map[string]string{
		"channels": `{"name":"Ops","channelType":"email","config":{}}`,
		"alerts":   `{"name":"Errors","channelId":1,"ruleType":"new_exception","config":{}}`,
		"teams":    `{"name":"Ops"}`, "schedules": `{"teamId":1,"name":"Primary","timezone":"UTC","definition":{}}`,
		"escalation-policies": `{"name":"Primary","definition":{}}`, "contact-methods": `{"methodType":"email","config":{}}`,
		"notification-chains": `{"high":[],"low":[]}`, "notification-history": `{"pagination":{"page":1,"pageSize":20}}`,
	}[resource])
}
func TestAlertingConfigurationValidation(t *testing.T) {
	for _, tc := range []struct {
		resource, action, body string
		valid                  bool
	}{
		{"channels", "create", `{"name":"Ops","channelType":"email","config":{},"enabled":false}`, true},
		{"contact-methods", "update", `{"enabled":false}`, true},
		{"contact-methods", "update", `{}`, false},
		{"contact-methods", "verify", `{"code":"not-a-code"}`, false},
		{"contact-methods", "verify", `{"code":"１２３４５６"}`, false},
		{"contact-methods", "verify", `{"code":"123456"}`, true},
		{"channels", "create", `{"name":123,"channelType":"email","config":{}}`, false},
		{"channels", "create", `{"name":"Ops","channelType":"email","config":"secret"}`, false},
		{"teams", "set-members", `{"userIds":[0]}`, false},
		{"notification-chains", "replace", `{"high":[],"low":[]}`, true},
		{"notification-chains", "replace", `{"high":[{"contactMethodId":1,"delayMInutes":5}],"low":[]}`, false},
		{"notification-chains", "replace", `{"high":[{"id":2,"contactMethodId":1,"delayMinutes":5}],"low":[]}`, true},
		{"notification-chains", "replace", `{"high":[]}`, false},
		{"notification-chains", "replace", `{"high":[{"contactMethodId":1,"delayMinutes":121}],"low":[]}`, false},
		{"notification-history", "list", `{"pagination":{"page":1,"pageSize":0}}`, false},
		{"teams", "create", `{"name":"Ops","unknown":true}`, false},
	} {
		req := AlertingRequest{ProjectID: "project", OrganizationID: "1", ID: 1, Body: json.RawMessage(tc.body)}
		if err := ValidateAlertingRequest(tc.resource, tc.action, req); (err == nil) != tc.valid {
			t.Errorf("%s %s %s valid=%v: %v", tc.resource, tc.action, tc.body, tc.valid, err)
		}
	}
	for _, req := range []AlertingRequest{{}, {ProjectID: "p", OrganizationID: "../1"}, {ProjectID: "p", OrganizationID: "0"}} {
		if err := ValidateAlertingRequest("teams", "list", req); err == nil {
			t.Errorf("accepted invalid request: %+v", req)
		}
	}
	if err := ValidateAlertingRequest("anything", "delete", AlertingRequest{ProjectID: "p"}); err == nil {
		t.Error("unallowlisted operation accepted")
	}
}
