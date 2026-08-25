package middleware

import (
	"compress/gzip"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
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
	reportBodyTotal = 2 * time.Minute
)

func useGzipLimited(c *gin.Context, maxBytes int64) {
	GuardBodyRead(c, reportBodyIdle, reportBodyTotal)

	// Decompress when Content-Encoding announces gzip; otherwise pass the
	// body through untouched. The pagehide / keepalive code path in the SDK
	// has to dispatch the request synchronously inside the unload handler,
	// which means it can't `await` the async CompressionStream — those
	// requests arrive as plain JSON and we accept them as-is.
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	if c.GetHeader("Content-Encoding") != "gzip" {
		c.Request.Body = body
		c.Next()
		return
	}

	gzReader, err := gzip.NewReader(body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid gzip"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, gzReader, maxBytes)
	c.Next()
}
