package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDeviceAuthorize_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/device/authorize" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"WXYZ-1234","verification_uri":"https://x/device","verification_uri_complete":"https://x/device?user_code=WXYZ-1234","expires_in":600,"interval":5}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	da, err := c.DeviceAuthorize(context.Background(), "")
	if err != nil {
		t.Fatalf("DeviceAuthorize: %v", err)
	}
	if da.DeviceCode != "dev" || da.UserCode != "WXYZ-1234" || da.Interval != 5 || da.ExpiresIn != 600 {
		t.Errorf("got %+v", da)
	}
}

func TestPollDeviceToken_states(t *testing.T) {
	cases := []struct {
		body string
		want error
	}{
		{`{"error":"authorization_pending"}`, ErrAuthorizationPending},
		{`{"error":"slow_down"}`, ErrSlowDown},
		{`{"error":"access_denied"}`, ErrAccessDenied},
		{`{"error":"expired_token"}`, ErrExpiredToken},
	}
	for _, tc := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(tc.body))
		}))
		c := New(srv.URL)
		_, err := c.PollDeviceToken(context.Background(), "dev")
		if !errors.Is(err, tc.want) {
			t.Errorf("body %s: got %v, want %v", tc.body, err, tc.want)
		}
		srv.Close()
	}
}

func TestPollDeviceToken_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"at","refresh_token":"rt","token_type":"Bearer","expires_in":900,"email":"a@b.com"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	ts, err := c.PollDeviceToken(context.Background(), "dev")
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if ts.AccessToken != "at" || ts.RefreshToken != "rt" || ts.Email != "a@b.com" || ts.ExpiresIn != 900 {
		t.Errorf("got %+v", ts)
	}
}

func TestRefresh_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/token" {
			t.Errorf("path = %q", r.URL.Path)
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["grant_type"] != "refresh_token" || req["refresh_token"] != "old" {
			t.Errorf("req = %v", req)
		}
		_, _ = w.Write([]byte(`{"access_token":"new-at","refresh_token":"new-rt","expires_in":900}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	ts, err := c.Refresh(context.Background(), "old")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if ts.AccessToken != "new-at" || ts.RefreshToken != "new-rt" {
		t.Errorf("got %+v", ts)
	}
}

func TestRefresh_invalidGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Refresh(context.Background(), "x")
	if !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("got %v, want ErrInvalidGrant", err)
	}
}

func TestDo_refreshOn401_retriesOnce(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		auth := r.Header.Get("Authorization")
		if n == 1 {
			if auth != "Bearer old" {
				t.Errorf("first auth = %q", auth)
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if auth != "Bearer new" {
			t.Errorf("retry auth = %q", auth)
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	refreshed := false
	c := New(srv.URL, WithJWT("old"), WithRefresher(func(_ context.Context) (string, error) {
		refreshed = true
		return "new", nil
	}))
	if _, err := c.ListProjects(context.Background()); err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if !refreshed {
		t.Error("refresher was not called")
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Errorf("server calls = %d, want 2", calls)
	}
}

func TestDo_refreshInvalidGrant_returnsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := New(srv.URL, WithJWT("old"), WithRefresher(func(_ context.Context) (string, error) {
		return "", ErrInvalidGrant
	}))
	_, err := c.ListProjects(context.Background())
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}

func TestDo_refreshFails_surfacesRefreshError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	refreshErr := errors.New("refresh failed")
	c := New(srv.URL, WithJWT("old"), WithRefresher(func(_ context.Context) (string, error) {
		return "", refreshErr
	}))
	_, err := c.ListProjects(context.Background())
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("got ErrUnauthorized, want the refresh error surfaced")
	}
	if !errors.Is(err, refreshErr) {
		t.Errorf("got %v, want wrapped %v", err, refreshErr)
	}
}
