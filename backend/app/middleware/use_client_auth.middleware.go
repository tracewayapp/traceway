package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var UseClientAuth func(c *gin.Context)

func InitUseClientAuth() {
	authHeader := "Bearer " + os.Getenv("TOKEN")

	UseClientAuth = func(c *gin.Context) {
		if authHeader != c.GetHeader("Authorization") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
