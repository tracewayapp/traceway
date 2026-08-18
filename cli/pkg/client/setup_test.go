package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSetupSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/setup/session" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tws_test" {
			t.Errorf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"organizationId":   3,
			"organizationName": "Acme",
			"email":            "a@b.c",
			"backendUrl":       "https://traceway.example.com",
			"projects":         []map[string]any{{"id": "p1", "name": "Api", "framework": "opentelemetry"}},
		})
	}))
	defer server.Close()

	session, err := New(server.URL, WithJWT("tws_test")).GetSetupSession(context.Background())
	if err != nil {
		t.Fatalf("GetSetupSession: %v", err)
	}
	if session.OrganizationName != "Acme" || session.Email != "a@b.c" || len(session.Projects) != 1 {
		t.Errorf("session = %+v", session)
	}
}

func TestSubmitSetupPlan(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/setup/plan" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "pending"})
	}))
	defer server.Close()

	err := New(server.URL, WithJWT("tws_test")).SubmitSetupPlan(context.Background(), []SetupPlanProject{
		{Name: "Api", Framework: "opentelemetry", EnvFile: "backend/.env", EnvVar: "TRACEWAY_TOKEN"},
	})
	if err != nil {
		t.Fatalf("SubmitSetupPlan: %v", err)
	}
	projects, ok := received["projects"].([]any)
	if !ok || len(projects) != 1 {
		t.Fatalf("request body = %v", received)
	}
	first := projects[0].(map[string]any)
	if first["name"] != "Api" || first["envVar"] != "TRACEWAY_TOKEN" {
		t.Errorf("projects[0] = %v", first)
	}
	if _, exists := first["deployment"]; exists {
		t.Error("empty deployment must be omitted from the wire body")
	}
}

func TestGetSetupPlanStatuses(t *testing.T) {
	responses := map[string]string{
		"pending":  `{"status":"pending"}`,
		"rejected": `{"status":"rejected","reason":"nope"}`,
		"approved": `{"status":"approved","projects":[{"id":"p1","name":"Api","framework":"opentelemetry","token":"tok_secret","backendUrl":"https://x.example","status":"created"}]}`,
	}
	for name, body := range responses {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			status, err := New(server.URL, WithJWT("tws_test")).GetSetupPlan(context.Background())
			if err != nil {
				t.Fatalf("GetSetupPlan: %v", err)
			}
			if status.Status != name {
				t.Errorf("status = %q, want %q", status.Status, name)
			}
			if name == "rejected" && status.Reason != "nope" {
				t.Errorf("reason = %q", status.Reason)
			}
			if name == "approved" && (len(status.Projects) != 1 || status.Projects[0].Token != "tok_secret") {
				t.Errorf("projects = %+v", status.Projects)
			}
		})
	}
}

func TestSetupEndpointsMapUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	c := New(server.URL, WithJWT("tws_expired"))
	if _, err := c.GetSetupSession(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("GetSetupSession error = %v, want ErrUnauthorized", err)
	}
	if _, err := c.GetSetupPlan(context.Background()); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("GetSetupPlan error = %v, want ErrUnauthorized", err)
	}
}
