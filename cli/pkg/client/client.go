package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client talks to a Traceway HTTP API. It is safe for concurrent use: JWT
// reads and the refresh path are serialized internally (long-lived consumers
// like the MCP server dispatch requests in parallel). Do not mutate JWT
// directly after construction; it is owned by the refresh path.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	JWT        string
	UserAgent  string
	refresher  Refresher

	// mu guards JWT and serializes the refresher so concurrent 401s cannot
	// double-spend a single-use refresh token: the losers of the race adopt
	// the winner's token instead of calling the refresher again.
	mu sync.Mutex
}

// Refresher returns a fresh access token (JWT) when the current one is
// rejected. do() invokes it at most once per request, on a 401, then retries.
// Invocations are serialized by the Client, so a Refresher never runs
// concurrently with itself on the same Client.
type Refresher func(ctx context.Context) (string, error)

// Option mutates a Client during construction.
type Option func(*Client)

// New returns a Client with sane defaults. The baseURL is normalized by
// stripping trailing slashes; do() prepends "/api/..." paths.
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "traceway-cli/0.1",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient injects a custom *http.Client (useful for tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.HTTPClient = h }
}

// WithJWT sets the bearer token to send on every request.
func WithJWT(jwt string) Option {
	return func(c *Client) { c.JWT = jwt }
}

func (c *Client) WithBearer(token string) *Client {
	return &Client{
		BaseURL:    c.BaseURL,
		HTTPClient: c.HTTPClient,
		JWT:        token,
		UserAgent:  c.UserAgent,
	}
}

// WithRefresher enables transparent access-token refresh: on a 401, do()
// calls the refresher for a new token and retries the request once.
func WithRefresher(r Refresher) Option {
	return func(c *Client) { c.refresher = r }
}

// do is the internal HTTP transport. It JSON-encodes body (if non-nil),
// JSON-decodes the response into out (if non-nil), and maps non-2xx status
// codes to typed errors.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var buf []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		buf = b
	}

	attempt := func(jwt string) (*http.Response, error) {
		var reqBody io.Reader
		if buf != nil {
			reqBody = bytes.NewReader(buf)
		}
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", c.UserAgent)
		if jwt != "" {
			req.Header.Set("Authorization", "Bearer "+jwt)
		}
		return c.HTTPClient.Do(req)
	}

	c.mu.Lock()
	jwt := c.JWT
	c.mu.Unlock()
	resp, err := attempt(jwt)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusUnauthorized && c.refresher != nil {
		resp.Body.Close()
		newJWT, refreshErr := c.refreshAfter401(ctx, jwt)
		if refreshErr != nil {
			// Only a dead session maps to ErrUnauthorized ("log in again").
			// Anything else (server error, network failure) must surface as
			// itself, or a transient outage reads as an expired login.
			if errors.Is(refreshErr, ErrUnauthorized) || errors.Is(refreshErr, ErrInvalidGrant) {
				return ErrUnauthorized
			}
			return fmt.Errorf("refreshing the session after a 401: %w", refreshErr)
		}
		if newJWT == "" {
			return ErrUnauthorized
		}
		resp, err = attempt(newJWT)
		if err != nil {
			return err
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		return nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	}
	respBody, _ := io.ReadAll(resp.Body)
	return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(respBody))}
}

// refreshAfter401 serializes token refresh across concurrent requests.
// rejected is the token the failed request presented: if another goroutine
// already swapped it for a fresh one, that token is adopted without spending
// the (single-use) refresh token again. The refresher runs while the lock is
// held, so concurrent 401s queue here and each sees the winner's result.
func (c *Client) refreshAfter401(ctx context.Context, rejected string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.JWT != "" && c.JWT != rejected {
		return c.JWT, nil
	}
	newJWT, err := c.refresher(ctx)
	if err != nil {
		return "", err
	}
	if newJWT != "" {
		c.JWT = newJWT
	}
	return newJWT, nil
}

// jsonMarshalNoHTMLEscape is a json.Marshal that doesn't escape <, >, & in
// strings. Used by request types with custom MarshalJSON to keep --output
// json human-readable. The default json.Marshal would otherwise turn
// "p < 5" into "p < 5" in the wire body.
func jsonMarshalNoHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder appends a trailing newline; trim it so callers can compose.
	out := buf.Bytes()
	if len(out) > 0 && out[len(out)-1] == '\n' {
		out = out[:len(out)-1]
	}
	return out, nil
}
