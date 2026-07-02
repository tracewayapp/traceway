package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"
	traceway "go.tracewayapp.com"

	"github.com/gin-gonic/gin"
)

const UserIdContextKey = "userId"
const UserEmailContextKey = "userEmail"

const patTouchInterval = time.Minute

func InitUseAppAuth() {
	UseAppAuth = func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		if strings.HasPrefix(tokenString, "twp_") {
			pat, err := repositories.PersonalAccessTokenRepository.FindActiveByToken(db.DB, tokenString)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("pat lookup failed: %w", err))
				return
			}
			if pat == nil {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.Set(UserIdContextKey, pat.UserId)
			c.Set(UserEmailContextKey, pat.Email)
			touchPersonalAccessToken(pat)
			c.Next()
			return
		}

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

func touchPersonalAccessToken(pat *repositories.ActivePAT) {
	now := time.Now()
	if pat.LastUsedAt != nil && now.Sub(*pat.LastUsedAt) < patTouchInterval {
		return
	}

	go func() {
		defer traceway.Recover()
		_, _ = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			return struct{}{}, repositories.PersonalAccessTokenRepository.TouchLastUsed(tx, pat.Id, now)
		})
	}()
}

var UseAppAuth func(c *gin.Context)

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
