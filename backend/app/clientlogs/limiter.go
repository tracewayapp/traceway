package clientlogs

import (
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

const Window = 30 * time.Second
const maxKeys = 65536
const ReviewMessage = "Client log volume exceeded: review this device or shared network"

type key struct {
	project uuid.UUID
	ip      string
}
type bucket struct {
	start   time.Time
	used    int
	flagged bool
}
type Limiter struct {
	mu      sync.Mutex
	entries map[key]bucket
	limit   int
	swept   time.Time
}

func New(limit int) *Limiter { return &Limiter{entries: make(map[key]bucket), limit: max(1, limit)} }

var Default = sync.OnceValue(func() *Limiter {
	limit := 1000
	if n, err := strconv.Atoi(os.Getenv("CLIENT_LOG_LIMIT_30S")); err == nil && n > 0 {
		limit = n
	}
	return New(limit)
})

func IsClient(project *models.Project) bool {
	if project == nil {
		return false
	}
	switch project.Framework {
	case "react", "svelte", "vuejs", "jquery", "react-native", "flutter", "ios", "android":
		return true
	}
	return false
}

// Allow uses server receipt time and a bounded map. Active keys are never evicted to make room for a new IP.
func (l *Limiter) Allow(project uuid.UUID, ip string, count int, now time.Time) (accepted int, flag bool, saturated bool) {
	if count <= 0 {
		return 0, false, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.swept) >= Window {
		for k, b := range l.entries {
			if now.Sub(b.start) >= Window {
				delete(l.entries, k)
			}
		}
		l.swept = now
	}
	k := key{project, canonicalIP(ip)}
	b, exists := l.entries[k]
	if !exists && len(l.entries) >= maxKeys {
		return 0, false, true
	}
	if !exists || now.Sub(b.start) >= Window {
		b = bucket{start: now}
	}
	accepted = min(count, l.limit-b.used)
	b.used += accepted
	flag = accepted < count && !b.flagged
	b.flagged = b.flagged || flag
	l.entries[k] = b
	return accepted, flag, false
}

func (l *Limiter) Review(project uuid.UUID, ip string, dropped int, session *uuid.UUID, attributes map[string]string) models.ExceptionStackTrace {
	attrs := map[string]string{}
	for _, key := range []string{"device.id", "device.model.name", "service.instance.id", "user.id", "user_agent.original"} {
		if value := attributes[key]; value != "" {
			attrs[key] = value
		}
	}
	attrs["client.ip"] = canonicalIP(ip)
	attrs["traceway.review.reason"] = "client_log_rate_limit"
	attrs["traceway.logs.dropped"] = strconv.Itoa(dropped)
	attrs["traceway.logs.limit_30s"] = strconv.Itoa(l.limit)
	sum := sha256.Sum256([]byte(ReviewMessage))
	return models.ExceptionStackTrace{Id: uuid.New(), ProjectId: project, ExceptionHash: hex.EncodeToString(sum[:8]), StackTrace: ReviewMessage, RecordedAt: time.Now().UTC(), Attributes: attrs, SessionId: session, IsMessage: true}
}

func canonicalIP(ip string) string {
	if parsed, err := netip.ParseAddr(ip); err == nil {
		return parsed.Unmap().String()
	}
	return "unknown"
}
