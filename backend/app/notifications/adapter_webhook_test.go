package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type webhookHit struct {
	method    string
	subject   string
	custom    string
	signature string
}

func webhookReceiver(hits chan<- webhookHit) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		_ = json.NewDecoder(r.Body).Decode(&payload)
		hits <- webhookHit{r.Method, payload["subject"], r.Header.Get("X-Custom"), r.Header.Get("X-Traceway-Signature")}
	}
}

func receivedHit(t *testing.T, hits <-chan webhookHit) webhookHit {
	t.Helper()
	select {
	case hit := <-hits:
		return hit
	default:
		t.Fatal("the receiver was never called")
		return webhookHit{}
	}
}

func redirectTo(code int, location string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", location)
		w.WriteHeader(code)
	}
}

func testWebhook(url string) *WebhookAdapter {
	return &WebhookAdapter{URL: url, Headers: map[string]string{"X-Custom": "token"}, Secret: "s3cret"}
}

func TestWebhookResendsPayloadOnHTTPSUpgrade(t *testing.T) {
	for _, code := range []int{301, 302, 303, 307, 308} {
		hits := make(chan webhookHit, 1)
		receiver := httptest.NewTLSServer(webhookReceiver(hits))
		defer receiver.Close()
		plain := httptest.NewServer(redirectTo(code, receiver.URL+"/hook"))
		defer plain.Close()

		transport := webhookClient.Transport
		webhookClient.Transport = receiver.Client().Transport
		err := testWebhook(plain.URL+"/hook").Send(context.Background(), Message{Subject: "upgrade"})
		webhookClient.Transport = transport
		if err != nil {
			t.Fatalf("%d: %v", code, err)
		}

		hit := receivedHit(t, hits)
		if hit.method != http.MethodPost || hit.subject != "upgrade" {
			t.Errorf("%d: the payload must be re-sent as a POST, got %s with subject %q", code, hit.method, hit.subject)
		}
		if hit.custom != "token" || hit.signature == "" {
			t.Errorf("%d: headers and signature must follow an upgrade on the same host, got %+v", code, hit)
		}
	}
}

func TestWebhookTreatsRedirectToAResponseAsDelivered(t *testing.T) {
	var echoed atomic.Int32
	echo := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { echoed.Add(1) }))
	defer echo.Close()
	otherHost := strings.Replace(echo.URL, "127.0.0.1", "localhost", 1)

	cases := []struct {
		code     int
		location string
	}{
		{http.StatusFound, otherHost + "/macros/echo"},
		{http.StatusSeeOther, otherHost + "/macros/echo"},
		{http.StatusFound, echo.URL + "/login"},
	}
	for _, tc := range cases {
		exec := httptest.NewServer(redirectTo(tc.code, tc.location))
		defer exec.Close()
		if err := testWebhook(exec.URL+"/exec").Send(context.Background(), Message{}); err != nil {
			t.Errorf("%d to %s: %v", tc.code, tc.location, err)
		}
	}
	if n := echoed.Load(); n != 0 {
		t.Errorf("the response behind a 302/303 must not be fetched, got %d requests", n)
	}
}

func TestWebhookStopsAfterMaxRedirects(t *testing.T) {
	var requests atomic.Int32
	var loop *httptest.Server
	loop = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		redirectTo(http.StatusTemporaryRedirect, loop.URL+r.URL.Path)(w, r)
	}))
	defer loop.Close()

	err := testWebhook(loop.URL+"/hook").Send(context.Background(), Message{})
	if err == nil || !strings.Contains(err.Error(), "redirected more than 3 times") {
		t.Fatalf("want the redirect cap error, got %v", err)
	}
	if n := requests.Load(); n != webhookMaxRedirects+1 {
		t.Errorf("want %d requests, got %d", webhookMaxRedirects+1, n)
	}
}

func TestWebhookKeepsHeadersOnTheChannelHost(t *testing.T) {
	hits := make(chan webhookHit, 1)
	receiver := httptest.NewServer(webhookReceiver(hits))
	defer receiver.Close()
	elsewhere := strings.Replace(receiver.URL, "127.0.0.1", "localhost", 1)
	moved := httptest.NewServer(redirectTo(http.StatusTemporaryRedirect, elsewhere+"/hook"))
	defer moved.Close()

	if err := testWebhook(moved.URL+"/hook").Send(context.Background(), Message{Subject: "moved"}); err != nil {
		t.Fatal(err)
	}
	hit := receivedHit(t, hits)
	if hit.subject != "moved" {
		t.Errorf("the payload must reach the new host, got %+v", hit)
	}
	if hit.custom != "" || hit.signature != "" {
		t.Errorf("headers and signature must not leave the channel's host, got %+v", hit)
	}
}

func TestWebhookSameSite(t *testing.T) {
	cases := []struct {
		host, origin string
		want         bool
	}{
		{"example.com", "example.com", true},
		{"www.example.com", "example.com", true},
		{"EXAMPLE.com", "example.com", true},
		{"evilexample.com", "example.com", false},
		{"example.com", "www.example.com", false},
	}
	for _, tc := range cases {
		if got := sameSite(tc.host, tc.origin); got != tc.want {
			t.Errorf("sameSite(%q, %q) = %v, want %v", tc.host, tc.origin, got, tc.want)
		}
	}
}
