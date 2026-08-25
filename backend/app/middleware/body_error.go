package middleware

import (
	"errors"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func BodyReadError(err error) (status int, message string, ok bool) {
	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytesErr):
		return http.StatusRequestEntityTooLarge, "Request body too large", true
	case errors.Is(err, ErrBodyTooSlow), errors.Is(err, os.ErrDeadlineExceeded):
		return http.StatusRequestTimeout, "Request body arrived too slowly", true
	}
	return 0, "", false
}

func RejectBindError(c *gin.Context, err error, fallback string) {
	status, message, ok := BodyReadError(err)
	if !ok {
		status, message = http.StatusBadRequest, fallback
	}
	c.JSON(status, gin.H{"error": message})
}
