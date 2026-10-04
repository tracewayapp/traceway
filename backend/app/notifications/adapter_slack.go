package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type SlackAdapter struct {
	WebhookURL string `json:"webhookUrl"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
}

func (a *SlackAdapter) Type() string { return "slack" }

func (a *SlackAdapter) Validate() error {
	if a.WebhookURL == "" {
		return fmt.Errorf("Slack webhook URL is required")
	}
	return nil
}

func (a *SlackAdapter) Send(ctx context.Context, msg Message) error {
	color := "#2196F3"
	switch msg.Severity {
	case SeverityWarning:
		color = "#FF9800"
	case SeverityCritical:
		color = "#F44336"
	}
	if msg.Recovered {
		color = "#4CAF50"
	}

	username := a.Username
	if username == "" {
		username = "Traceway"
	}

	payload := map[string]interface{}{
		"username": username,
		"attachments": []map[string]interface{}{
			{
				"color":  color,
				"title":  msg.Subject,
				"text":   msg.Body,
				"footer": "Traceway Alerts",
				"ts":     time.Now().Unix(),
			},
		},
	}

	if a.Channel != "" {
		payload["channel"] = a.Channel
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create Slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Slack request failed: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("Slack returned status %d", resp.StatusCode)
	}

	return nil
}
