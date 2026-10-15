package mcpserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestListPages_defaultsToActive(t *testing.T) {
	var body map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pages" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":20,"total":0,"totalPages":0}}`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "list_pages", nil)
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(t, res))
	}
	if body["status"] != "active" {
		t.Errorf("status = %v", body["status"])
	}
	if res.StructuredContent == nil {
		t.Error("pass-through tools must return structured content")
	}
}

func TestPassThrough_wrapsArrays(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1}]`))
	})
	cs := connect(t, backend, "p-1")
	res := callTool(t, cs, "list_notification_rules", nil)
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(t, res))
	}
	m, ok := res.StructuredContent.(map[string]any)
	if !ok || m["data"] == nil {
		t.Errorf("an array result must be wrapped under data, got %#v", res.StructuredContent)
	}
}

func TestUpdatePostMortem_mergesWithCurrent(t *testing.T) {
	var put map[string]any
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"id":3,"title":"Keep me","contentMd":"old","tags":["a"],"incidentId":null,"updatedAt":"2026-10-15T10:05:00Z"}`))
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&put)
		_, _ = w.Write([]byte(`{"id":3,"title":"Keep me","contentMd":"new"}`))
	})
	cs := connectWrite(t, backend, "p-1")
	res := callTool(t, cs, "update_post_mortem", map[string]any{"id": 3, "content_md": "new", "expected_updated_at": "2026-10-15T10:00:00Z"})
	if res.IsError {
		t.Fatalf("tool error: %s", resultText(t, res))
	}
	if put["title"] != "Keep me" || put["contentMd"] != "new" {
		t.Errorf("PUT body = %v", put)
	}
	if put["expectedUpdatedAt"] != "2026-10-15T10:00:00Z" {
		t.Errorf("expectedUpdatedAt = %v, want the revision the agent read", put["expectedUpdatedAt"])
	}
}

func TestUpdatePostMortem_contentNeedsExpectedUpdatedAt(t *testing.T) {
	called := false
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{}`))
	})
	cs := connectWrite(t, backend, "p-1")
	res := callTool(t, cs, "update_post_mortem", map[string]any{"id": 3, "content_md": "new"})
	if !res.IsError || !strings.Contains(resultText(t, res), "expected_updated_at") || called {
		t.Errorf("want an invalid_argument error before any request, got %s (called %v)", resultText(t, res), called)
	}
}

func TestWriteTools_surfaceForbiddenMessage(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Agent write access is turned off for this organization."}`))
	})
	cs := connectWrite(t, backend, "p-1")
	res := callTool(t, cs, "acknowledge_page", map[string]any{"id": 4})
	if !res.IsError || !strings.Contains(resultText(t, res), "turned off") || !strings.Contains(resultText(t, res), "error=forbidden") {
		t.Errorf("want a forbidden tool error carrying the reason, got %s", resultText(t, res))
	}
}

func TestReadServer_hasNoOperationsResource(t *testing.T) {
	read := connect(t, emptyBackend(), "p-1")
	write := connectWrite(t, emptyBackend(), "p-1")
	has := func(uri string, list []string) bool {
		for _, u := range list {
			if u == uri {
				return true
			}
		}
		return false
	}
	readRes, err := read.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	writeRes, err := write.ListResources(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var readURIs, writeURIs []string
	for _, r := range readRes.Resources {
		readURIs = append(readURIs, r.URI)
	}
	for _, r := range writeRes.Resources {
		writeURIs = append(writeURIs, r.URI)
	}
	if has("traceway://knowledge/operations", readURIs) || !has("traceway://knowledge/operations", writeURIs) {
		t.Errorf("operations resource: read %v, write %v", readURIs, writeURIs)
	}
	for _, uri := range []string{"traceway://knowledge/learn", "traceway://knowledge/learn/on-call"} {
		if !has(uri, readURIs) || !has(uri, writeURIs) {
			t.Errorf("both servers carry %s: read %v, write %v", uri, readURIs, writeURIs)
		}
	}
}
