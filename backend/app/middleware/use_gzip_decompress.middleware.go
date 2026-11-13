package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/v2/app/config"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
)

const defaultReportMaxBodyMB = 64

var reportMaxBodyBytes = sync.OnceValue(func() int64 {
	value := ""
	if config.Config != nil {
		value = config.Config.ReportMaxBodyMB
	}
	return config.SizeMB(value, defaultReportMaxBodyMB)
})

func UseGzip(c *gin.Context) {
	useGzipLimited(c, reportMaxBodyBytes())
}

const (
	reportBodyIdle  = 20 * time.Second
	reportBodyTotal = DefaultBodyTotal
)

func useGzipLimited(c *gin.Context, maxBytes int64) {
	GuardBodyRead(c, reportBodyIdle, reportBodyTotal)

	meter := &ingestBodyMeter{wire: c.Request.Body, limit: maxBytes}
	c.Set(ingestBodyMeterKey, meter)
	defer meter.record(c)

	// Decompress when Content-Encoding announces gzip; otherwise pass the
	// body through untouched. The pagehide / keepalive code path in the SDK
	// has to dispatch the request synchronously inside the unload handler,
	// which means it can't `await` the async CompressionStream — those
	// requests arrive as plain JSON and we accept them as-is.
	body := http.MaxBytesReader(c.Writer, &meteredBody{rc: c.Request.Body, bytes: &meter.wireBytes, spent: &meter.wireTime}, maxBytes)
	if c.GetHeader("Content-Encoding") != "gzip" {
		c.Request.Body = body
		c.Next()
		return
	}

	// The gzip header is read here, so this waits for the client's first bytes.
	gzReader, err := gzip.NewReader(body)
	if err != nil {
		RejectIngestBindError(c, err, "Invalid gzip")
		c.Abort()
		return
	}
	meter.gzip = true
	c.Request.Body = http.MaxBytesReader(c.Writer, &meteredBody{rc: gzReader, bytes: &meter.decodedBytes, spent: &meter.decodedTime}, maxBytes)
	c.Next()
}

const ingestBodyMeterKey = "ingestBodyMeter"

// ingestBodyMeter splits the time an ingest handler spends on its body into
// waiting for the client (reads of the wire) and decompressing, which a span
// around the handler's read cannot tell apart.
type ingestBodyMeter struct {
	wire  io.ReadCloser
	limit int64
	gzip  bool

	wireBytes    int64
	wireTime     time.Duration
	decodedBytes int64
	decodedTime  time.Duration
}

type meteredBody struct {
	rc    io.ReadCloser
	bytes *int64
	spent *time.Duration
}

func (m *meteredBody) Read(b []byte) (int, error) {
	start := time.Now()
	n, err := m.rc.Read(b)
	*m.spent += time.Since(start)
	*m.bytes += int64(n)
	return n, err
}

func (m *meteredBody) Close() error { return m.rc.Close() }

func (m *ingestBodyMeter) record(c *gin.Context) {
	traceway.SetAttribute(c, "ingest.body.wire_bytes", strconv.FormatInt(m.wireBytes, 10))
	traceway.SetAttribute(c, "ingest.body.network_ms", strconv.FormatInt(m.wireTime.Milliseconds(), 10))
	if !m.gzip {
		return
	}
	traceway.SetAttribute(c, "ingest.body.decoded_bytes", strconv.FormatInt(m.decodedBytes, 10))
	traceway.SetAttribute(c, "ingest.body.gunzip_ms", strconv.FormatInt(max(m.decodedTime-m.wireTime, 0).Milliseconds(), 10))
}

// discardRest reads what the client is still sending, without keeping it, so
// a response written afterwards reaches a client that only reads it once its
// upload is complete. It gives up past the body cap or on a read error.
func (m *ingestBodyMeter) discardRest() bool {
	metered := &meteredBody{rc: m.wire, bytes: &m.wireBytes, spent: &m.wireTime}
	n, err := io.Copy(io.Discard, io.LimitReader(metered, m.limit+1))
	return err == nil && n <= m.limit
}
