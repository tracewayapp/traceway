package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/outbox"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func githubRequests(t *testing.T, msg Message) []string {
	t.Helper()
	var bodies []string
	transport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})
	defer func() { http.DefaultTransport = transport }()

	adapter := &GitHubAdapter{Token: "t", Owner: "acme", Repo: "alerts"}
	if err := adapter.Send(context.Background(), msg); err != nil {
		t.Fatalf("send to GitHub: %v", err)
	}
	return bodies
}

func TestGitHubRecoveryOpensNoIssue(t *testing.T) {
	check := &models.SyntheticCheck{Id: 7, Name: "api health"}

	down := githubRequests(t, buildCheckDownMessage(check, "timeout", "api"))
	if len(down) != 1 || !strings.Contains(down[0], "is down") {
		t.Fatalf("check down: requests = %q, want one issue for the outage", down)
	}

	if recovered := githubRequests(t, buildCheckRecoveredMessage(check, "api")); len(recovered) != 0 {
		t.Errorf("recovery: requests = %q, want none", recovered)
	}
}

func TestGitHubIssueStaysWithinGitHubLimits(t *testing.T) {
	bodies := githubRequests(t, Message{Subject: strings.Repeat("é", 400), Body: strings.Repeat("b", 70000)})
	if len(bodies) != 1 {
		t.Fatalf("requests = %d, want 1", len(bodies))
	}
	var issue struct{ Title, Body string }
	if err := json.Unmarshal([]byte(bodies[0]), &issue); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	if n := len([]rune(issue.Title)); n != 256 {
		t.Errorf("title length = %d, want 256", n)
	}
	if n := len([]rune(issue.Body)); n != 65536 {
		t.Errorf("body length = %d, want 65536", n)
	}
}

func TestGitHubRejectionSaysWhyAndIsNotRetried(t *testing.T) {
	respond := func(status int, body string) error {
		transport := http.DefaultTransport
		http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		defer func() { http.DefaultTransport = transport }()
		adapter := &GitHubAdapter{Token: "t", Owner: "acme", Repo: "alerts"}
		return adapter.Send(context.Background(), Message{Subject: "s", Body: "b"})
	}

	err := respond(http.StatusUnprocessableEntity, `{"message":"Validation Failed","errors":[{"resource":"Issue","field":"title","code":"too_long"}]}`)
	var permanent *outbox.PermanentError
	if !errors.As(err, &permanent) {
		t.Fatalf("422 = %v, want a permanent error", err)
	}
	if want := "GitHub returned status 422: Validation Failed (title too_long)"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}

	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests, http.StatusBadGateway} {
		err := respond(status, "upstream")
		if err == nil || errors.As(err, &permanent) {
			t.Errorf("status %d = %v, want a retryable error", status, err)
		}
	}
}
