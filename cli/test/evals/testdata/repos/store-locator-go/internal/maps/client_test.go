package maps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeocodeAndDistances(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/geocode":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"location": map[string]float64{"lat": 52.53, "lng": 13.38}}}})
		case "/v1/distancematrix":
			_ = json.NewEncoder(w).Encode(map[string]any{"rows": []any{map[string]int{"meters": 1200}, map[string]int{"meters": 3400}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "k")
	p, err := c.Geocode(context.Background(), "10115 Berlin")
	if err != nil || p.Lat != 52.53 {
		t.Fatalf("geocode: %v %v", p, err)
	}
	m, err := c.Distances(context.Background(), p, []string{"store:1", "store:2"})
	if err != nil || len(m) != 2 || m[1] != 3400 {
		t.Fatalf("distances: %v %v", m, err)
	}
}
