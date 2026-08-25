package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const maxAuthBodyBytes = 64 << 10

const maxTransactionalBodyBytes = 8 << 20

const bodyBufferedContextKey = "bodyBuffered"

func BufferBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bufferRequestBody(c, maxBytes) {
			c.Next()
		}
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
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxBytesErr):
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "Request body too large"})
		case errors.Is(err, ErrBodyTooSlow), errors.Is(err, os.ErrDeadlineExceeded):
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{"error": "Request body arrived too slowly"})
		default:
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Could not read request body"})
		}
		return false
	}

	c.Request.Body = io.NopCloser(bytes.NewReader(data))
	c.Request.ContentLength = int64(len(data))
	c.Set(bodyBufferedContextKey, true)
	return true
}

func BufferAuthBody(c *gin.Context) {
	bufferAuthBody(c)
}

var bufferAuthBody = BufferBody(maxAuthBodyBytes)
