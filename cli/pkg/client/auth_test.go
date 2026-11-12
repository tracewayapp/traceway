package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestLogin_success(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q", r.Method)
		}
		var req map[string]string
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["email"] != "fred@example.com" {
			t.Errorf("email = %q", req["email"])
		}
		if req["password"] != "hunter2" {
			t.Errorf("password = %q", req["password"])
		}
		_, _ = w.Write([]byte(`{"token": "jwt.value.here"}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	jwt, err := c.Login(context.Background(), "fred@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if jwt != "jwt.value.here" {
		t.Errorf("jwt = %q", jwt)
	}
}

func TestLogin_invalidCredentials_returnsUnauthorized(t *testing.T) {
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Login(context.Background(), "x", "y")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("got %v, want ErrUnauthorized", err)
	}
}
