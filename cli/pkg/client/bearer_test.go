package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithBearer_presentsOwnTokenWithoutRefresher(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer derived" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	refreshed := false
	base := New(srv.URL, WithJWT("base"), WithRefresher(func(context.Context) (string, error) {
		refreshed = true
		return "base-refreshed", nil
	}))

	derived := base.WithBearer("derived")
	if derived.BaseURL != base.BaseURL || derived.HTTPClient != base.HTTPClient {
		t.Error("derived client should share base URL and HTTP transport")
	}
	if _, err := derived.ListProjects(context.Background()); err != nil {
		t.Fatalf("derived request failed: %v", err)
	}
	if gotAuth != "Bearer derived" {
		t.Errorf("Authorization = %q, want the derived token", gotAuth)
	}
	if base.JWT != "base" {
		t.Errorf("base client JWT changed to %q", base.JWT)
	}

	rejected := base.WithBearer("wrong")
	if _, err := rejected.ListProjects(context.Background()); err != ErrUnauthorized {
		t.Errorf("401 on a derived client should map to ErrUnauthorized without refreshing, got %v", err)
	}
	if refreshed {
		t.Error("derived client must not inherit the base refresher")
	}
}
