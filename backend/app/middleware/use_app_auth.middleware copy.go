package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

var UseAppAuth func(c *gin.Context)

func InitUseAppAuth() {
	authHeader := "Bearer " + os.Getenv("TOKEN")

	UseAppAuth = func(c *gin.Context) {
		if authHeader != c.GetHeader("Authorization") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
