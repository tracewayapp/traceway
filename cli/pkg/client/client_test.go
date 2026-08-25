package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_setsDefaults(t *testing.T) {
	c := New("https://example.com")
	if c.BaseURL != "https://example.com" {
		t.Errorf("BaseURL = %q", c.BaseURL)
	}
	if c.HTTPClient == nil {
		t.Error("HTTPClient should default to a non-nil client")
	}
	if c.UserAgent == "" {
		t.Error("UserAgent should have a default value")
	}
}

func TestWithJWT_setsJWT(t *testing.T) {
	c := New("https://example.com", WithJWT("tok"))
	if c.JWT != "tok" {
		t.Errorf("JWT = %q", c.JWT)
	}
}

func TestDo_setsHeadersAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "traceway-cli") {
			t.Errorf("User-Agent = %q", got)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["foo"] != "bar" {
			t.Errorf("body.foo = %q", body["foo"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("tok"))
	var resp struct {
		OK bool `json:"ok"`
	}
	err := c.do(context.Background(), http.MethodPost, "/api/test", map[string]string{"foo": "bar"}, &resp)
	if err != nil {
		t.Fatalf("do(): %v", err)
	}
	if !resp.OK {
		t.Error("expected resp.OK = true")
	}
}

func TestDo_omitsAuthHeaderWhenJWTEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("expected no Authorization header, got %q", got)
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	var resp struct{}
	if err := c.do(context.Background(), http.MethodPost, "/api/x", nil, &resp); err != nil {
		t.Fatal(err)
	}
}

func TestDo_mapsStatusCodes(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{401, ErrUnauthorized},
		{403, ErrForbidden},
		{404, ErrNotFound},
		{429, ErrRateLimited},
	}
	for _, c := range cases {
		c := c
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
			}))
			defer srv.Close()

			cli := New(srv.URL)
			var resp struct{}
			err := cli.do(context.Background(), http.MethodPost, "/api/x", nil, &resp)
			if !errors.Is(err, c.want) {
				t.Errorf("got %v, want errors.Is(_, %v)", err, c.want)
			}
		})
	}
}

func TestDo_returnsAPIErrorForOtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	cli := New(srv.URL)
	var resp struct{}
	err := cli.do(context.Background(), http.MethodPost, "/api/x", nil, &resp)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d", apiErr.StatusCode)
	}
	if apiErr.Body != "boom" {
		t.Errorf("Body = %q", apiErr.Body)
	}
}
