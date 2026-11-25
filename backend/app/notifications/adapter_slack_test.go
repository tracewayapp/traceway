package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func sentSlackColor(t *testing.T, msg Message) string {
	t.Helper()
	type attachment struct {
		Color string `json:"color"`
	}
	attachments := make(chan []attachment, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Attachments []attachment `json:"attachments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		attachments <- payload.Attachments
	}))
	defer receiver.Close()

	if err := (&SlackAdapter{WebhookURL: receiver.URL}).Send(context.Background(), msg); err != nil {
		t.Fatalf("send to Slack: %v", err)
	}
	got := <-attachments
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
	return got[0].Color
}

// The recovery rows run on every build tag; the dispatch test that covers the
// same color end to end only builds on the default tags.
func TestSlackColor(t *testing.T) {
	recovered := buildCheckRecoveredMessage(&models.SyntheticCheck{Id: 7, Name: "api health"}, "api")
	relabelled := recovered
	relabelled.Severity = SeverityCritical
	cases := []struct {
		name string
		msg  Message
		want string
	}{
		{"critical", Message{Severity: SeverityCritical}, "#F44336"},
		{"warning", Message{Severity: SeverityWarning}, "#FF9800"},
		{"info", Message{Severity: SeverityInfo}, "#2196F3"},
		{"no severity", Message{}, "#2196F3"},
		{"recovery", recovered, "#4CAF50"},
		{"recovery with a critical severity", relabelled, "#4CAF50"},
	}
	for _, tc := range cases {
		if got := sentSlackColor(t, tc.msg); got != tc.want {
			t.Errorf("%s: color = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func sentSlackTitleLink(t *testing.T, msg Message) (string, bool) {
	t.Helper()
	links := make(chan map[string]interface{}, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Attachments []map[string]interface{} `json:"attachments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		links <- payload.Attachments[0]
	}))
	defer receiver.Close()

	if err := (&SlackAdapter{WebhookURL: receiver.URL}).Send(context.Background(), msg); err != nil {
		t.Fatalf("send to Slack: %v", err)
	}
	link, ok := (<-links)["title_link"].(string)
	return link, ok
}

// Issue #390: only the exception body carried a link, so every other alert
// reached Slack with no way back to the dashboard.
func TestSlackTitleLink(t *testing.T) {
	down := buildCheckDownMessage(&models.SyntheticCheck{Id: 1, Name: "local tcp"}, "connection refused", "api")

	absolute := down
	absolute.URL = "https://traceway.example.com/monitors/1"
	if link, _ := sentSlackTitleLink(t, absolute); link != absolute.URL {
		t.Errorf("title_link = %q, want %q", link, absolute.URL)
	}

	if link, ok := sentSlackTitleLink(t, down); ok {
		t.Errorf("title_link = %q for the relative %q, want it unset", link, down.URL)
	}
}
