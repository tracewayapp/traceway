package middleware

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

// An ingest slot bounds the memory of decoding and storing a batch, but a slot
// taken before the body is read is also held while the client uploads it. In
// production a phone on a weak signal held one for 20 seconds and a 12 MB
// replay batch for two minutes, with the CPU idle, so a burst of ordinary
// exports queued behind them. Reading a body whose length is declared into
// memory before admission keeps uploads out of the slots: the slot then covers
// only the work it exists to bound. The bytes read ahead are capped by a
// process-wide budget; a body over the budget's share, one without a declared
// length, or one arriving while the budget is spent is read under the slot as
// before.

const defaultIngestPrefetchBudgetMB = 64

// A single request may take at most this fraction of the budget, so one large
// upload cannot shut every other request out of the read-ahead.
const ingestPrefetchRequestShare = 4

const ingestPrefetchKey = "ingestPrefetch"

type byteBudget struct {
	limit int64
	used  atomic.Int64
}

func (b *byteBudget) tryAcquire(n int64) bool {
	for {
		used := b.used.Load()
		if used+n > b.limit {
			return false
		}
		if b.used.CompareAndSwap(used, used+n) {
			return true
		}
	}
}

func (b *byteBudget) release(n int64) { b.used.Add(-n) }

type prefetchedBody struct {
	bytes   int64
	network time.Duration
}

func ingestPrefetchBudgetFromEnv() *byteBudget {
	mb := int64(defaultIngestPrefetchBudgetMB)
	if v := os.Getenv("INGEST_PREFETCH_BUDGET_MB"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed >= 0 {
			mb = parsed
		}
	}
	if mb == 0 {
		return nil
	}
	return &byteBudget{limit: mb << 20}
}

// Built on first use for the same reason as the admission gate: the .env file
// is loaded after package variables initialize.
var ingestPrefetchBudget = sync.OnceValue(ingestPrefetchBudgetFromEnv)

// PrefetchIngestBody returns a middleware, placed in front of an admission
// gate, that reads the body ahead of it under the given guard budget. maxBytes
// is the route's cap on the raw body; a declared length above it is left to the
// handler, which rejects it as it always has.
func PrefetchIngestBody(idle, total time.Duration, maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		prefetchIngestBody(c, ingestPrefetchBudget(), idle, total, maxBytes)
	}
}

// PrefetchReportBody is PrefetchIngestBody for the routes behind UseGzip.
func PrefetchReportBody(c *gin.Context) {
	prefetchIngestBody(c, ingestPrefetchBudget(), reportBodyIdle, reportBodyTotal, reportMaxBodyBytes())
}

func prefetchIngestBody(c *gin.Context, budget *byteBudget, idle, total time.Duration, maxBytes int64) {
	if budget == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
		c.Next()
		return
	}
	length := c.Request.ContentLength
	switch {
	case length <= 0:
		skipPrefetch(c, "no_length")
		return
	case length > maxBytes || length > budget.limit/ingestPrefetchRequestShare:
		skipPrefetch(c, "too_large")
		return
	case !budget.tryAcquire(length):
		skipPrefetch(c, "budget_full")
		return
	}
	defer budget.release(length)
	traceway.CaptureMetricMax("traceway.ingest.prefetch.budget_used_pcnt", 100*float64(budget.used.Load())/float64(budget.limit), nil)

	GuardBodyRead(c, idle, total)
	_, span := traceway.StartSpan(c, "ingest.read_body")
	start := time.Now()
	data := make([]byte, length)
	read, err := io.ReadFull(c.Request.Body, data)
	network := time.Since(start)
	span.End()

	traceway.SetAttribute(c, "ingest.body.prefetched", "true")
	traceway.SetAttribute(c, "ingest.body.wire_bytes", strconv.Itoa(read))
	traceway.SetAttribute(c, "ingest.body.network_ms", strconv.FormatInt(network.Milliseconds(), 10))
	traceway.CaptureMetricMax("traceway.ingest.body.network_ms", float64(network.Milliseconds()), nil)
	if err != nil {
		RejectIngestBindError(c, err, "Could not read request body")
		c.Abort()
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Set(ingestPrefetchKey, &prefetchedBody{bytes: length, network: network})
	c.Next()
}

func skipPrefetch(c *gin.Context, reason string) {
	traceway.SetAttribute(c, "ingest.body.prefetch_skipped", reason)
	traceway.CaptureCount("traceway.ingest.prefetch.skipped", 1, map[string]string{"reason": reason})
	c.Next()
}

func prefetchedNetworkTime(c *gin.Context) time.Duration {
	if v, ok := c.Get(ingestPrefetchKey); ok {
		if p, ok := v.(*prefetchedBody); ok {
			return p.network
		}
	}
	return 0
}

// MeterRawIngestBody times the reads of an ingest body that was not read
// ahead, so the request span says how long the client took to send it. The
// returned function records the result; it does nothing for a prefetched body,
// whose upload the prefetch already timed.
func MeterRawIngestBody(c *gin.Context) (record func()) {
	if _, ok := c.Get(ingestPrefetchKey); ok || c.Request.Body == nil || c.Request.Body == http.NoBody {
		return func() {}
	}
	var read int64
	var spent time.Duration
	c.Request.Body = &meteredBody{rc: c.Request.Body, bytes: &read, spent: &spent}
	return func() {
		traceway.SetAttribute(c, "ingest.body.wire_bytes", strconv.FormatInt(read, 10))
		traceway.SetAttribute(c, "ingest.body.network_ms", strconv.FormatInt(spent.Milliseconds(), 10))
		traceway.CaptureMetricMax("traceway.ingest.body.network_ms", float64(spent.Milliseconds()), nil)
	}
}
