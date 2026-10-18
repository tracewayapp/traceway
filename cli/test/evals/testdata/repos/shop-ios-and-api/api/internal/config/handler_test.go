package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeStore struct{}

func (fakeStore) Flags(context.Context, string) (map[string]bool, error) {
	return map[string]bool{"new_home": true}, nil
}

func (fakeStore) Versions(context.Context, string) (string, string, error) {
	return "4.8.0", "4.12.0", nil
}

func TestHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	Handler(fakeStore{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config?platform=ios", nil))
	var p Payload
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Flags["new_home"] || p.MinimumVersion != "4.8.0" || p.LatestVersion != "4.12.0" {
		t.Fatalf("payload %+v", p)
	}
}
