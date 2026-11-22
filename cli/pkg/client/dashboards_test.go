package client

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCreateDashboardEmbed(t *testing.T) {
	var body map[string]any
	var request string
	srv := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request = r.Method + " " + r.URL.Path + "?" + r.URL.RawQuery
		body = nil
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"embedPath":"/embed/dashboard?dashboard=d-1#token=jwt","token":"jwt"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, WithJWT("t"))

	raw, err := c.CreateDashboardEmbed(t.Context(), "p-1", 42, DashboardEmbedOptions{}, "https://traceway.example.com/")
	var out map[string]any
	if err != nil || json.Unmarshal(raw, &out) != nil {
		t.Fatal(err)
	}
	if request != "POST /api/dashboards/42/embed-token?projectId=p-1" || len(body) != 0 {
		t.Fatalf("default link: %s %v", request, body)
	}
	if out["url"] != "https://traceway.example.com/embed/dashboard?dashboard=d-1#token=jwt" || out["token"] != "jwt" {
		t.Fatalf("response: %v", out)
	}

	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	raw, err = c.CreateDashboardEmbed(t.Context(), "p-1", 42, DashboardEmbedOptions{
		From: from, To: from.AddDate(0, 0, 7), ExpiresAt: from.AddDate(0, 1, 0), TagFilters: map[string]string{"account_id": "acme"}, Theme: "dark",
	}, "")
	out = nil
	if err != nil || json.Unmarshal(raw, &out) != nil {
		t.Fatal(err)
	}
	filters, _ := body["tagFilters"].(map[string]any)
	if request != "POST /api/dashboards/42/embed-token?projectId=p-1&theme=dark" || body["from"] != "2026-11-01T00:00:00Z" || body["to"] != "2026-11-08T00:00:00Z" || body["expiresAt"] != "2026-12-01T00:00:00Z" || filters["account_id"] != "acme" {
		t.Fatalf("fixed link: %s %v", request, body)
	}
	if _, ok := out["url"]; ok {
		t.Fatal("a link without an origin has no url")
	}
}

func TestDashboardEmbedOptionsValidate(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, valid := range []DashboardEmbedOptions{
		{}, {Timeframe: "30m"}, {Timeframe: "last42h"}, {Timeframe: "731d", ExpiresInDays: DashboardEmbedMaxDays},
		{From: at, To: at.Add(time.Hour), ExpiresAt: at.AddDate(0, 0, 3)}, {TagFilters: map[string]string{"account_id": "acme"}, Theme: "light"},
	} {
		if err := valid.Validate(); err != nil {
			t.Errorf("%+v: %v", valid, err)
		}
	}
	for _, invalid := range []DashboardEmbedOptions{
		{Timeframe: "week"}, {Timeframe: "0d"}, {Timeframe: "-1h"}, {Timeframe: "7d", From: at, To: at.Add(time.Hour)},
		{From: at}, {To: at}, {From: at.Add(time.Hour), To: at},
		{ExpiresInDays: -1}, {ExpiresInDays: DashboardEmbedMaxDays + 1}, {ExpiresInDays: 3, ExpiresAt: at},
		{Theme: "sepia"}, {TagFilters: map[string]string{"": "acme"}}, {TagFilters: map[string]string{"account_id": ""}},
	} {
		if err := invalid.Validate(); err == nil {
			t.Errorf("accepted %+v", invalid)
		}
	}
}
