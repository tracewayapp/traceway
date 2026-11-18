package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestListExceptions_sendsProjectIdAsQueryParam(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId query = %q", r.URL.Query().Get("projectId"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["fromDate"] == nil || body["toDate"] == nil {
			t.Errorf("body missing fromDate/toDate: %v", body)
		}
		_, _ = w.Write([]byte(`{
			"data": [
				{"exceptionHash":"h1","stackTrace":"...","count":42,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"}
			],
			"pagination":{"page":1,"pageSize":50,"total":1,"totalPages":1}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.ListExceptions(context.Background(), "proj-1", ListExceptionsRequest{
		TimeRange:  TimeRange{From: time.Now().Add(-time.Hour), To: time.Now()},
		Pagination: PaginationParams{Page: 1, PageSize: 50},
	})
	if err != nil {
		t.Fatalf("ListExceptions: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("got %d exceptions, want 1", len(resp.Data))
	}
	if resp.Data[0].ExceptionHash != "h1" {
		t.Errorf("ExceptionHash = %q", resp.Data[0].ExceptionHash)
	}
	if resp.Data[0].Count != 42 {
		t.Errorf("Count = %d", resp.Data[0].Count)
	}
	if resp.Pagination.Total != 1 {
		t.Errorf("Pagination.Total = %d", resp.Pagination.Total)
	}
}

func TestListExceptions_emptyResults(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"pagination":{"page":1,"pageSize":50,"total":0,"totalPages":0}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.ListExceptions(context.Background(), "proj-1", ListExceptionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty Data")
	}
}

func TestListExceptions_passesSearchParameters(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["search"] != "NullPointer" {
			t.Errorf("search = %v", body["search"])
		}
		if body["searchType"] != "text" {
			t.Errorf("searchType = %v", body["searchType"])
		}
		if body["includeArchived"] != true {
			t.Errorf("includeArchived = %v", body["includeArchived"])
		}
		_, _ = w.Write([]byte(`{"data":[],"pagination":{}}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	_, err := c.ListExceptions(context.Background(), "proj-1", ListExceptionsRequest{
		Search:          "NullPointer",
		SearchType:      "text",
		IncludeArchived: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGetException_sendsHashInPath(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/exception-stack-traces/abc123" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("projectId") != "proj-1" {
			t.Errorf("projectId = %q", r.URL.Query().Get("projectId"))
		}
		_, _ = w.Write([]byte(`{
			"group": {"exceptionHash":"abc123","stackTrace":"trace","count":7,"firstSeen":"2026-05-13T00:00:00Z","lastSeen":"2026-05-13T01:00:00Z"},
			"occurrences": [
				{"id":"00000000-0000-0000-0000-000000000001","exceptionHash":"abc123","stackTrace":"trace","recordedAt":"2026-05-13T00:30:00Z"}
			],
			"pagination": {"page":1,"pageSize":20,"total":1,"totalPages":1}
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	resp, err := c.GetException(context.Background(), "proj-1", "abc123", PaginationParams{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("GetException: %v", err)
	}
	if resp.Group == nil || resp.Group.ExceptionHash != "abc123" {
		t.Errorf("Group missing or wrong hash: %+v", resp.Group)
	}
	if len(resp.Occurrences) != 1 {
		t.Fatalf("got %d occurrences, want 1", len(resp.Occurrences))
	}
	if resp.Occurrences[0].ExceptionHash != "abc123" {
		t.Errorf("Occurrence ExceptionHash = %q", resp.Occurrences[0].ExceptionHash)
	}
}

func TestGetException_404_returnsErrNotFound(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	_, err := c.GetException(context.Background(), "proj-1", "missing", PaginationParams{})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestArchiveExceptions_postsHashesAndProjectId(t *testing.T) {
	var gotPath string
	var gotProject string
	var gotBody map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.ArchiveExceptions(context.Background(), "proj-1", []string{"h1", "h2", "h3"})
	if err != nil {
		t.Fatalf("ArchiveExceptions: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/archive" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	hashes, _ := gotBody["hashes"].([]any)
	if len(hashes) != 3 || hashes[0] != "h1" || hashes[2] != "h3" {
		t.Errorf("body hashes = %v", hashes)
	}
}

func TestArchiveExceptions_403_returnsErrForbidden(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.ArchiveExceptions(context.Background(), "proj-1", []string{"h1"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestUnarchiveExceptions_postsHashesAndProjectId(t *testing.T) {
	var gotPath string
	var gotProject string
	var gotBody map[string]any
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotProject = r.URL.Query().Get("projectId")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	err := c.UnarchiveExceptions(context.Background(), "proj-1", []string{"h1"})
	if err != nil {
		t.Fatalf("UnarchiveExceptions: %v", err)
	}
	if gotPath != "/api/exception-stack-traces/unarchive" {
		t.Errorf("path = %q", gotPath)
	}
	if gotProject != "proj-1" {
		t.Errorf("projectId = %q", gotProject)
	}
	hashes, _ := gotBody["hashes"].([]any)
	if len(hashes) != 1 || hashes[0] != "h1" {
		t.Errorf("body hashes = %v", hashes)
	}
}
