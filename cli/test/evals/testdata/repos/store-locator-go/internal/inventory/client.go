// Package inventory reads store stock from the internal inventory service.
package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client is the inventory client.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client using rt (the instrumented default transport).
func New(baseURL string, rt http.RoundTripper) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 2 * time.Second, Transport: rt}}
}

// Stock returns the units on hand per SKU for one store.
func (c *Client) Stock(ctx context.Context, storeID int) (map[string]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/v2/stock?store=%d", c.baseURL, storeID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out map[string]int
	return out, json.NewDecoder(resp.Body).Decode(&out)
}
