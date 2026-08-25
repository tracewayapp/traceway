package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestOAuthTokenRateLimitRejectsInTheRFCShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/token", RateLimitOAuthTokenPerIP(1, time.Minute), func(c *gin.Context) { c.Status(http.StatusOK) })

	post := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/token", nil)
		req.RemoteAddr = "203.0.113.5:1234"
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := post(); rec.Code != http.StatusOK {
		t.Fatalf("first request got %d, want 200", rec.Code)
	}
	rec := post()
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"slow_down"`) {
		t.Fatalf("over the limit got %d %s, want 400 with error slow_down", rec.Code, rec.Body.String())
	}
}
