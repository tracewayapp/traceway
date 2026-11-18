// Package env runs one hermetic Traceway instance per eval run: the
// standalone backend binary in SQLite mode in a temp dir, plus the users,
// projects, tokens and CLI logins a case needs. It talks HTTP only and never
// imports the backend.
package env

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// API is a small JSON client for the dashboard API.
type API struct {
	Base   string // http://127.0.0.1:<port>, no /api suffix
	Bearer string
	HTTP   *http.Client
}

// Response is a decoded API reply.
type Response struct {
	Status int
	Body   []byte
}

// JSON decodes the body into v.
func (r *Response) JSON(v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("decode %d response: %w (body: %s)", r.Status, err, clip(r.Body))
	}
	return nil
}

// Map decodes the body as an object.
func (r *Response) Map() map[string]any {
	var m map[string]any
	_ = json.Unmarshal(r.Body, &m)
	return m
}

// OK reports a 2xx status.
func (r *Response) OK() bool { return r.Status >= 200 && r.Status < 300 }

// Err returns an error describing a non-2xx reply, or nil.
func (r *Response) Err(what string) error {
	if r.OK() {
		return nil
	}
	return fmt.Errorf("%s: HTTP %d: %s", what, r.Status, clip(r.Body))
}

// WithBearer returns a copy authenticated with another token.
func (a *API) WithBearer(token string) *API {
	c := *a
	c.Bearer = token
	return &c
}

// Do sends a request. path starts with /api/...; query may be nil.
func (a *API) Do(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	u := a.Base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if a.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+a.Bearer)
	}
	hc := a.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Body: raw}, nil
}

// MustOK sends a request and fails on a non-2xx status.
func (a *API) MustOK(ctx context.Context, method, path string, query url.Values, body any) (*Response, error) {
	r, err := a.Do(ctx, method, path, query, body)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	return r, r.Err(method + " " + path)
}

// ProjectQuery returns ?projectId=<id>.
func ProjectQuery(projectID string) url.Values {
	return url.Values{"projectId": []string{projectID}}
}

func clip(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}
