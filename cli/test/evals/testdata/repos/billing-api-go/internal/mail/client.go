// Package mail is the client of the mailer's HTTP API.
package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/billing-api/internal/telemetry"
)

// Message is one templated e-mail.
type Message struct {
	To       string            `json:"to"`
	Template string            `json:"template"`
	Data     map[string]string `json:"data"`
}

// Client sends messages through the mailer.
type Client struct {
	base string
	key  string
	http *http.Client
}

// New returns a Client for the mailer API at base.
func New(base, key string) *Client {
	return &Client{base: base, key: key, http: &http.Client{Transport: telemetry.Transport}}
}

// Send posts one message; the mailer queues it and answers 202.
func (c *Client) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v3/messages", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("mailer: %s", resp.Status)
	}
	return nil
}
