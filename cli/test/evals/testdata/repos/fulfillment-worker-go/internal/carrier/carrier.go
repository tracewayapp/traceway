// Package carrier is the carrier's label API client.
package carrier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"example.com/fulfillment/internal/labels"
	"example.com/fulfillment/internal/telemetry"
)

// Client calls the carrier API.
type Client struct {
	base string
	key  string
	http *http.Client
}

// New returns a Client.
func New(base, key string) *Client {
	return &Client{base: base, key: key, http: &http.Client{Timeout: 15 * time.Second, Transport: telemetry.Transport}}
}

// CreateLabel buys a label.
func (c *Client) CreateLabel(ctx context.Context, req labels.LabelRequest) (*labels.Label, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v2/labels", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+c.key)
	r.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("POST %s/v2/labels: %s", c.base, resp.Status)
	}
	var l labels.Label
	if err := json.NewDecoder(resp.Body).Decode(&l); err != nil {
		return nil, err
	}
	return &l, nil
}
