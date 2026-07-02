package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimitPerIP returns a fixed-window per-IP rate limiter for unauthenticated
// endpoints that persist state per request (e.g. the device-authorize endpoint,
// which inserts a main-DB row per call). Buckets live in memory; stale ones are
// swept on each request, so the map stays bounded by the number of distinct IPs
// seen within one window.
func RateLimitPerIP(maxRequests int, window time.Duration) gin.HandlerFunc {
	type bucket struct {
		windowStart time.Time
		count       int
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}

	return func(c *gin.Context) {
		now := time.Now()

		mu.Lock()
		for ip, b := range buckets {
			if now.Sub(b.windowStart) > window {
				delete(buckets, ip)
			}
		}
		ip := c.ClientIP()
		b, ok := buckets[ip]
		if !ok {
			b = &bucket{windowStart: now}
			buckets[ip] = b
		}
		b.count++
		count := b.count
		mu.Unlock()

		if count > maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "slow_down"})
			return
		}
		c.Next()
	}
}
