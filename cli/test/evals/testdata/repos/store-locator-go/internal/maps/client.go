// Package maps is the client for the maps provider (geocoding and driving
// distances).
package maps

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"example.com/store-locator/internal/telemetry"
)

// Client calls the maps provider.
type Client struct {
	baseURL string
	apiKey  string
}

// New returns a client for the provider at baseURL.
func New(baseURL, apiKey string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey}
}

// Point is a WGS84 coordinate.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Geocode resolves a free-text query (postcode, address) to a point.
func (c *Client) Geocode(ctx context.Context, q string) (Point, error) {
	var out struct {
		Results []struct {
			Location Point `json:"location"`
		} `json:"results"`
	}
	if err := c.get(ctx, "/v1/geocode?q="+url.QueryEscape(q)+"&region=de", &out); err != nil {
		return Point{}, err
	}
	if len(out.Results) == 0 {
		return Point{}, fmt.Errorf("maps: no result for %q", q)
	}
	return out.Results[0].Location, nil
}

// Distances returns the driving distance in meters from origin to each
// destination, in order.
func (c *Client) Distances(ctx context.Context, origin Point, destinations []string) ([]int, error) {
	var out struct {
		Rows []struct {
			Meters int `json:"meters"`
		} `json:"rows"`
	}
	path := fmt.Sprintf("/v1/distancematrix?origins=%.4f,%.4f&destinations=%s&mode=driving",
		origin.Lat, origin.Lng, url.QueryEscape(strings.Join(destinations, "|")))
	if err := c.get(ctx, path, &out); err != nil {
		return nil, err
	}
	meters := make([]int, len(out.Rows))
	for i, r := range out.Rows {
		meters[i] = r.Meters
	}
	return meters, nil
}

func (c *Client) get(ctx context.Context, path string, into any) error {
	hc := &http.Client{
		Timeout: 3 * time.Second,
		Transport: telemetry.Transport(&http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			TLSHandshakeTimeout: 2 * time.Second,
		}),
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("maps: %s answered %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}
