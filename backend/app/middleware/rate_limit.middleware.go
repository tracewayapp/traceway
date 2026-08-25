package middleware

import (
	"net/http"
	"strconv"
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
	return b.count <= l.maxRequests
}

func rateLimitWithKey(maxRequests int, window time.Duration, keyOf func(c *gin.Context) string) gin.HandlerFunc {
	limiter := NewFixedWindowLimiter(maxRequests, window)
	return func(c *gin.Context) {
		if !limiter.Allow(keyOf(c)) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "slow_down"})
			return
		}
		c.Next()
	}
}

func RateLimitOAuthTokenPerIP(maxRequests int, window time.Duration) gin.HandlerFunc {
	limiter := NewFixedWindowLimiter(maxRequests, window)
	return func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "slow_down"})
			return
		}
		c.Next()
	}
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
