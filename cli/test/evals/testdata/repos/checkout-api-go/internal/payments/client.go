// Package payments is the client for the payment provider.
package payments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client talks to the payment provider's REST API.
type Client struct {
	BaseURL string
	APIKey  string
	http    *http.Client
}

// New returns a client for the provider at baseURL, sending through rt.
func New(baseURL, apiKey string, rt http.RoundTripper) *Client {
	return &Client{BaseURL: baseURL, APIKey: apiKey, http: &http.Client{
		Timeout:   10 * time.Second,
		Transport: rt,
	}}
}

// Charge is the provider's charge object.
type Charge struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ChargeRequest charges an order total to a saved payment method.
type ChargeRequest struct {
	AmountMinor    int64  `json:"amount"`
	Currency       string `json:"currency"`
	PaymentMethod  string `json:"payment_method"`
	IdempotencyKey string `json:"-"`
}

// Charge creates a charge.
func (c *Client) Charge(ctx context.Context, req ChargeRequest) (*Charge, error) {
	var out Charge
	return &out, c.post(ctx, "/v1/charges", req.IdempotencyKey, req, &out)
}

// Refund refunds a charge in full.
func (c *Client) Refund(ctx context.Context, chargeID, idempotencyKey string) error {
	return c.post(ctx, "/v1/refunds", idempotencyKey, map[string]string{"charge": chargeID}, nil)
}

func (c *Client) post(ctx context.Context, path, idem string, body, into any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	if idem != "" {
		req.Header.Set("Idempotency-Key", idem)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("payments: %s answered %d", path, resp.StatusCode)
	}
	if into == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
