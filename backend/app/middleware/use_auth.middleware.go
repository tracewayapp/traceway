package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var UseAuth func(c *gin.Context)

func InitUseAuth() {
	authHeader := "Bearer " + os.Getenv("TOKEN")

	UseAuth = func(c *gin.Context) {
		if authHeader != c.GetHeader("Authorization") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
