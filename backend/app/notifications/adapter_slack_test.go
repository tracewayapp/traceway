package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type slackAttachment struct {
	Color string `json:"color"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

func sentSlackAttachment(t *testing.T, msg Message) slackAttachment {
	t.Helper()
	attachments := make(chan []slackAttachment, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Attachments []slackAttachment `json:"attachments"`
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
	return got[0]
}

func TestSlackColorBySeverity(t *testing.T) {
	cases := []struct {
		severity Severity
		want     string
	}{
		{SeverityCritical, "#F44336"},
		{SeverityWarning, "#FF9800"},
		{SeverityInfo, "#2196F3"},
		{"", "#2196F3"},
	}
	for _, tc := range cases {
		if got := sentSlackAttachment(t, Message{Subject: "s", Severity: tc.severity}).Color; got != tc.want {
			t.Errorf("severity %q: colour = %s, want %s", tc.severity, got, tc.want)
		}
	}
}
