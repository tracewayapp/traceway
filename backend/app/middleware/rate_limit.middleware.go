package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// FixedWindowLimiter is a fixed-window in-memory rate limiter keyed by an
// arbitrary string (IP, user id, phone number, ...). Stale buckets are swept
// on each call, so the map stays bounded by the number of distinct keys seen
// within one window.
type FixedWindowLimiter struct {
	mu          sync.Mutex
	maxRequests int
	window      time.Duration
	buckets     map[string]*limiterBucket
}

type limiterBucket struct {
	windowStart time.Time
	count       int
}

func NewFixedWindowLimiter(maxRequests int, window time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		maxRequests: maxRequests,
		window:      window,
		buckets:     map[string]*limiterBucket{},
	}
}

// Allow consumes one slot for key and reports whether the request is within
// the limit.
func (l *FixedWindowLimiter) Allow(key string) bool {
	allowed, _ := l.Take(key)
	return allowed
}

func (l *FixedWindowLimiter) Take(key string) (bool, time.Duration) {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.windowStart) > l.window {
			delete(l.buckets, k)
		}
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &limiterBucket{windowStart: now}
		l.buckets[key] = b
	}
	b.count++
	if b.count <= l.maxRequests {
		return true, 0
	}
	return false, b.windowStart.Add(l.window).Sub(now)
}

func rejectRateLimited(c *gin.Context, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	c.Header("Retry-After", strconv.Itoa(seconds))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "slow_down"})
}

func rateLimitWithKey(maxRequests int, window time.Duration, keyOf func(c *gin.Context) string) gin.HandlerFunc {
	limiter := NewFixedWindowLimiter(maxRequests, window)
	return func(c *gin.Context) {
		allowed, retryAfter := limiter.Take(keyOf(c))
		if !allowed {
			rejectRateLimited(c, retryAfter)
			return
		}
		c.Next()
	}
}

func RateLimitOAuthTokenPerIP(maxRequests int, window time.Duration) gin.HandlerFunc {
	limiter := NewFixedWindowLimiter(maxRequests, window)
	return func(c *gin.Context) {
		allowed, retryAfter := limiter.Take(c.ClientIP())
		if allowed {
			c.Next()
			return
		}
		grant, ok := oauthGrantType(c)
		if !ok {
			return
		}
		if grant == "device_code" || grant == "urn:ietf:params:oauth:grant-type:device_code" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "slow_down"})
			return
		}
		rejectRateLimited(c, retryAfter)
	}
}

func oauthGrantType(c *gin.Context) (string, bool) {
	if !bufferRequestBody(c, maxAuthBodyBytes) {
		return "", false
	}
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return "", true
	}
	data, err := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return "", true
	}
	if strings.HasPrefix(c.ContentType(), "application/json") {
		var body struct {
			GrantType string `json:"grant_type"`
		}
		_ = json.Unmarshal(data, &body)
		return body.GrantType, true
	}
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return "", true
	}
	return values.Get("grant_type"), true
}

// RateLimitPerIP returns a fixed-window per-IP rate limiter for unauthenticated
// endpoints that persist state per request (e.g. the device-authorize endpoint,
// which inserts a main-DB row per call).
func RateLimitPerIP(maxRequests int, window time.Duration) gin.HandlerFunc {
	return rateLimitWithKey(maxRequests, window, func(c *gin.Context) string {
		return c.ClientIP()
	})
}

// RateLimitPerUser keys the limit by the authenticated user (UseAppAuth must
// run first), so users behind a shared NAT don't exhaust each other's budget
// and an attacker cannot widen theirs by rotating IPs. Requests without a
// resolved user fall back to the client IP so the limit still holds.
func RateLimitPerUser(maxRequests int, window time.Duration) gin.HandlerFunc {
	return rateLimitWithKey(maxRequests, window, func(c *gin.Context) string {
		if userId := GetUserId(c); userId != 0 {
			return "u:" + strconv.Itoa(userId)
		}
		return "ip:" + c.ClientIP()
	})
}
