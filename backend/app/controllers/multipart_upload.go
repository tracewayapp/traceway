package controllers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
)

const (
	maxUploadBytes        = 250 << 20
	uploadMultipartMemory = 8 << 20
	uploadBodyIdle        = 30 * time.Second
	uploadBodyTotal       = 30 * time.Minute
)

type countingReadCloser struct {
	rc io.ReadCloser
	n  int64
}

func (c *countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingReadCloser) Close() error { return c.rc.Close() }

func parseMultipartCapped(c *gin.Context, maxBytes, maxMemory int64) bool {
	middleware.GuardBodyRead(c, uploadBodyIdle, uploadBodyTotal)

	if c.Request.ContentLength > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Total upload size exceeds %dMB", maxBytes>>20)})
		return false
	}

	counter := &countingReadCloser{rc: c.Request.Body}
	c.Request.Body = http.MaxBytesReader(c.Writer, counter, maxBytes)

	err := c.Request.ParseMultipartForm(maxMemory)
	if err == nil {
		if _, err = io.Copy(io.Discard, c.Request.Body); err == nil {
			return true
		}
		c.Request.MultipartForm.RemoveAll()
	}

	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr) || counter.n > maxBytes:
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Total upload size exceeds %dMB", maxBytes>>20)})
	case errors.Is(err, multipart.ErrMessageTooLarge):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Too many files in one request; split the upload into smaller batches"})
	default:
		if status, message, ok := middleware.BodyReadError(err); ok {
			c.JSON(status, gin.H{"error": message})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		}
	}
	return false
}
