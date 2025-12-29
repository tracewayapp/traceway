package middleware

import (
	"compress/gzip"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UseGzip(c *gin.Context) {
	if c.GetHeader("Content-Encoding") != "gzip" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	gzReader, err := gzip.NewReader(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid gzip"})
		return
	}
	c.Request.Body = gzReader
	c.Next()
}
