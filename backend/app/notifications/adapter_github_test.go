package notifications

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
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
