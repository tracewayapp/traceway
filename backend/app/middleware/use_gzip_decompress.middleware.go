package middleware

import (
	"compress/gzip"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

var gzipRequestReaderPool sync.Pool

func UseGzip(c *gin.Context) {
	// Decompress when Content-Encoding announces gzip; otherwise pass the
	// body through untouched. The pagehide / keepalive code path in the SDK
	// has to dispatch the request synchronously inside the unload handler,
	// which means it can't `await` the async CompressionStream — those
	// requests arrive as plain JSON and we accept them as-is.
	if c.GetHeader("Content-Encoding") != "gzip" {
		c.Next()
		return
	}

	var gzReader *gzip.Reader
	var err error
	if pooled, ok := gzipRequestReaderPool.Get().(*gzip.Reader); ok {
		gzReader = pooled
		err = gzReader.Reset(c.Request.Body)
	} else {
		gzReader, err = gzip.NewReader(c.Request.Body)
	}
	if err != nil {
		if gzReader != nil {
			gzipRequestReaderPool.Put(gzReader)
		}
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid gzip"})
		return
	}
	c.Request.Body = gzReader
	c.Next()
	// The handler chain is synchronous and done with the body here; nothing
	// retains it past the request, so the reader can be reused.
	gzReader.Close()
	gzipRequestReaderPool.Put(gzReader)
}
