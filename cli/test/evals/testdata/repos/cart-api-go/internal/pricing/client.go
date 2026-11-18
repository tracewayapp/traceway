// Package pricing is the client of the pricing service, which prices a
// cart: shipping, tax and promotions.
package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"example.com/cart-api/internal/config"
	"example.com/cart-api/internal/telemetry"
)

// Quote is the priced cart.
type Quote struct {
	SubtotalCents int64  `json:"subtotalCents"`
	ShippingCents int64  `json:"shippingCents"`
	TaxCents      int64  `json:"taxCents"`
	DiscountCents int64  `json:"discountCents"`
	TotalCents    int64  `json:"totalCents"`
	Currency      string `json:"currency"`
}

// Client calls the pricing service.
type Client struct {
	base string
	http *http.Client
}

// New returns a Client configured from cfg.
func New(cfg config.Config) *Client {
	return &Client{base: cfg.PricingURL, http: &http.Client{Timeout: cfg.PricingTimeout, Transport: telemetry.Transport(http.DefaultTransport)}}
}

// Quote prices a cart.
func (c *Client) Quote(ctx context.Context, cartID string) (*Quote, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/quotes?cart="+url.QueryEscape(cartID), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pricing: GET /v1/quotes: %s", resp.Status)
	}
	var q Quote
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		return nil, fmt.Errorf("pricing: decode quote: %w", err)
	}
	return &q, nil
}
