package client

import (
	"context"
	"net/http"
	"testing"
)

func TestListProjects_decodesData(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q", r.Method)
		}
		_, _ = w.Write([]byte(`[
			{"id": "p1", "name": "stormwind-prod"},
			{"id": "p2", "name": "stormwind-staging"}
		]`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	projects, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(projects))
	}
	if projects[0].ID != "p1" {
		t.Errorf("projects[0].ID = %q", projects[0].ID)
	}
	if projects[1].Name != "stormwind-staging" {
		t.Errorf("projects[1].Name = %q", projects[1].Name)
	}
}
