package middleware

import (
	"backend/app/services"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const UserIdContextKey = "userId"
const UserEmailContextKey = "userEmail"

var UseAppAuth func(c *gin.Context)

func InitUseAppAuth() {
	UseAppAuth = func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := services.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set(UserIdContextKey, claims.UserId)
		c.Set(UserEmailContextKey, claims.Email)

		c.Next()
	}
}

func GetUserId(c *gin.Context) int {
	if id, exists := c.Get(UserIdContextKey); exists {
		return id.(int)
	}
	return 0
}

func GetUserEmail(c *gin.Context) string {
	if email, exists := c.Get(UserEmailContextKey); exists {
		return email.(string)
	}
	return ""
}
