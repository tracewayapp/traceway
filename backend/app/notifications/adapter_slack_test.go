package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tracewayapp/traceway/backend/app/models"
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
