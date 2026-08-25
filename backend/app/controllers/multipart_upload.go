package controllers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
)

const (
	sourceMapMaxUploadBytes = 250 << 20
	symbolsMaxUploadBytes   = 250 << 20

	sourceMapMaxFiles = 500
	symbolsMaxFiles   = 100

	uploadBodyIdle  = 30 * time.Second
	uploadBodyTotal = 30 * time.Minute

	uploadMultipartMemory = 8 << 20
)

func checkFileCount(c *gin.Context, files int, max int, noun string) bool {
	if files > max {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("a maximum of %d %s can be uploaded per request; split the upload into smaller batches", max, noun)})
		return false
	}
	return true
}

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
	}

	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr) || counter.n > maxBytes:
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("Total upload size exceeds %dMB", maxBytes>>20)})
	case errors.Is(err, os.ErrDeadlineExceeded):
		c.JSON(http.StatusRequestTimeout, gin.H{"error": "Upload timed out before the whole body arrived"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
	}
	return false
}
