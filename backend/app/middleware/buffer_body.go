package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

const maxAuthBodyBytes = 64 << 10

const maxTransactionalBodyBytes = 8 << 20

const bodyBufferedContextKey = "bodyBuffered"

func BufferAuthBody(c *gin.Context) {
	if bufferRequestBody(c, maxAuthBodyBytes) {
		c.Next()
	}
}

func bufferRequestBody(c *gin.Context, maxBytes int64) bool {
	if c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
		return true
	}
	if c.GetBool(bodyBufferedContextKey) {
		return true
	}

	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes))
	if err != nil {
		status, message, ok := BodyReadError(err)
		if !ok {
			status, message = http.StatusBadRequest, "Could not read request body"
		}
		c.AbortWithStatusJSON(status, gin.H{"error": message})
		return false
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	c.Set(bodyBufferedContextKey, true)
	return true
}
