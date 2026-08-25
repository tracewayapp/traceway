package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func tokenRouter(limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/token", RateLimitOAuthTokenPerIP(limit, time.Minute), func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func postToken(r *gin.Engine, contentType, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.5:1234"
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	r.ServeHTTP(rec, req)
	return rec
}

func TestOAuthTokenRateLimitAnswersDevicePollsInTheRFCShape(t *testing.T) {
	for _, tc := range []struct{ name, contentType, body string }{
		{"json short grant", "application/json", `{"grant_type":"device_code","device_code":"x"}`},
		{"json urn grant", "application/json", `{"grant_type":"urn:ietf:params:oauth:grant-type:device_code","device_code":"x"}`},
		{"form urn grant", "application/x-www-form-urlencoded", "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Adevice_code&device_code=x"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := tokenRouter(1)
			if rec := postToken(r, tc.contentType, tc.body); rec.Code != http.StatusOK {
				t.Fatalf("first request got %d, want 200", rec.Code)
			}
			rec := postToken(r, tc.contentType, tc.body)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"slow_down"`) {
				t.Fatalf("over the limit got %d %s, want 400 with error slow_down", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestOAuthTokenRateLimitAnswersOtherGrantsWith429(t *testing.T) {
	for _, tc := range []struct{ name, contentType, body string }{
		{"json refresh", "application/json", `{"grant_type":"refresh_token","refresh_token":"x"}`},
		{"form refresh", "application/x-www-form-urlencoded", "grant_type=refresh_token&refresh_token=x"},
		{"form code exchange", "application/x-www-form-urlencoded", "grant_type=authorization_code&code=x"},
		{"no body", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := tokenRouter(1)
			if rec := postToken(r, tc.contentType, tc.body); rec.Code != http.StatusOK {
				t.Fatalf("first request got %d, want 200", rec.Code)
			}
			rec := postToken(r, tc.contentType, tc.body)
			if rec.Code != http.StatusTooManyRequests {
				t.Fatalf("over the limit got %d %s, want 429", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("429 must carry Retry-After")
			}
		})
	}
}

func TestRateLimitPerIPCarriesRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", RateLimitPerIP(1, time.Minute), func(c *gin.Context) { c.Status(http.StatusOK) })
	post := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		r.ServeHTTP(rec, req)
		return rec
	}
	post()
	rec := post()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" || got == "0" {
		t.Fatalf("Retry-After = %q, want the seconds until the window resets", got)
	}
}
