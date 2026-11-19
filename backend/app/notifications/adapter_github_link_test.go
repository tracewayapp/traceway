package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

type githubIssueStub func(*http.Request) (*http.Response, error)

func (f githubIssueStub) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func sentGitHubIssueBody(t *testing.T, msg Message) string {
	t.Helper()
	var body string
	transport := http.DefaultTransport
	http.DefaultTransport = githubIssueStub(func(r *http.Request) (*http.Response, error) {
		var payload struct {
			Body string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		body = payload.Body
		return &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})
	defer func() { http.DefaultTransport = transport }()

	if err := (&GitHubAdapter{Token: "t", Owner: "acme", Repo: "alerts"}).Send(context.Background(), msg); err != nil {
		t.Fatalf("send to GitHub: %v", err)
	}
	return body
}

// Issue #390: only the exception body carried a link, so every other alert
// opened an issue with no way back to the dashboard.
func TestGitHubIssueLink(t *testing.T) {
	down := buildCheckDownMessage(&models.SyntheticCheck{Id: 1, Name: "local tcp"}, "connection refused", "api")
	link := "https://traceway.example.com/monitors/1"

	absolute := down
	absolute.URL = link
	if got := sentGitHubIssueBody(t, absolute); !strings.HasSuffix(got, "[View in Traceway]("+link+")") {
		t.Errorf("body does not end with the dashboard link:\n%s", got)
	}

	if got := sentGitHubIssueBody(t, down); got != down.Body {
		t.Errorf("body changed for the relative %q:\n%s", down.URL, got)
	}

	exception := buildNewErrorMessage(ExceptionDetails{Hash: "0123456789abcdef", ErrorType: "RuntimeError"}, "api")
	exception.URL = "https://traceway.example.com/issues/0123456789abcdef"
	exception.Body += "\n\nView details: " + exception.URL
	if got := sentGitHubIssueBody(t, exception); strings.Count(got, exception.URL) != 1 {
		t.Errorf("want the link once, body:\n%s", got)
	}
}
