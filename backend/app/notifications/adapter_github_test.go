package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type githubCall struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func githubServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, *[]githubCall) {
	t.Helper()
	calls := &[]githubCall{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]interface{}
		json.Unmarshal(raw, &body)
		*calls = append(*calls, githubCall{Method: r.Method, Path: r.URL.Path, Body: body})
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func TestCreateIssueReturnsIssueNumber(t *testing.T) {
	server, calls := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"number":17,"html_url":"https://github.com/acme/backend/issues/17"}`))
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", Labels: []string{"bug"}, baseURL: server.URL}
	number, err := adapter.CreateIssue(context.Background(), Message{Subject: "New error", Body: "trace"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if number != 17 {
		t.Errorf("number = %d, expected 17", number)
	}
	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	call := (*calls)[0]
	if call.Method != http.MethodPost || call.Path != "/repos/acme/backend/issues" {
		t.Errorf("wrong request: %s %s", call.Method, call.Path)
	}
	if call.Body["title"] != "New error" || call.Body["body"] != "trace" {
		t.Errorf("wrong payload: %v", call.Body)
	}
}

func TestCreateIssueSucceedsWithUnreadableBody(t *testing.T) {
	server, _ := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`not json`))
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	number, err := adapter.CreateIssue(context.Background(), Message{Subject: "New error"})
	// The issue exists: a retry would open a duplicate, so this must not fail.
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if number != 0 {
		t.Errorf("number = %d, expected 0 so the caller skips recording", number)
	}
}

func TestCreateIssueFailsOnErrorStatus(t *testing.T) {
	server, _ := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	if _, err := adapter.CreateIssue(context.Background(), Message{Subject: "New error"}); err == nil {
		t.Fatal("expected an error so the outbox retries")
	}
}

func TestCloseIssueClosesThenComments(t *testing.T) {
	server, calls := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	if err := adapter.CloseIssue(context.Background(), "acme", "backend", 17, "archived in Traceway"); err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected a close and a comment, got %d calls", len(*calls))
	}
	closeCall, commentCall := (*calls)[0], (*calls)[1]
	if closeCall.Method != http.MethodPatch || closeCall.Path != "/repos/acme/backend/issues/17" {
		t.Errorf("wrong close request: %s %s", closeCall.Method, closeCall.Path)
	}
	if closeCall.Body["state"] != "closed" || closeCall.Body["state_reason"] != "completed" {
		t.Errorf("wrong close payload: %v", closeCall.Body)
	}
	if commentCall.Method != http.MethodPost || commentCall.Path != "/repos/acme/backend/issues/17/comments" {
		t.Errorf("wrong comment request: %s %s", commentCall.Method, commentCall.Path)
	}
	if commentCall.Body["body"] != "archived in Traceway" {
		t.Errorf("wrong comment payload: %v", commentCall.Body)
	}
}

func TestCloseIssueUsesTheRecordedRepository(t *testing.T) {
	server, calls := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// The channel has been repointed since the issue was opened; the close must
	// still reach the repository the issue actually lives in.
	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "new-repo", baseURL: server.URL}
	if err := adapter.CloseIssue(context.Background(), "acme", "old-repo", 17, ""); err != nil {
		t.Fatalf("close: %v", err)
	}
	if path := (*calls)[0].Path; path != "/repos/acme/old-repo/issues/17" {
		t.Errorf("closed the wrong issue: %s", path)
	}
}

func TestCloseIssueTreatsMissingIssueAsClosed(t *testing.T) {
	server, _ := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	// Retrying cannot bring a deleted issue back, so this must not keep failing.
	if err := adapter.CloseIssue(context.Background(), "acme", "backend", 17, "archived"); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestCloseIssueFailsOnErrorStatus(t *testing.T) {
	server, _ := githubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	adapter := &GitHubAdapter{Token: "ghp_x", Owner: "acme", Repo: "backend", baseURL: server.URL}
	if err := adapter.CloseIssue(context.Background(), "acme", "backend", 17, "archived"); err == nil {
		t.Fatal("expected an error so the outbox retries")
	}
}
