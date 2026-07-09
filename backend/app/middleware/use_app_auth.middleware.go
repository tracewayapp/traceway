package middleware

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
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

		identity, err := AuthenticateBearer(strings.TrimPrefix(authHeader, "Bearer "))
		if err != nil {
			if errors.Is(err, ErrInvalidBearer) {
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("bearer auth failed: %w", err))
			return
		}

		c.Set(UserIdContextKey, identity.UserId)
		c.Set(UserEmailContextKey, identity.Email)

		c.Next()
	}
}

var ErrInvalidBearer = errors.New("invalid bearer token")

type BearerIdentity struct {
	UserId  int
	Email   string
	Expires time.Time
}

func AuthenticateBearer(tokenString string) (*BearerIdentity, error) {
	if strings.HasPrefix(tokenString, "twp_") {
		pat, err := transactional.PersonalAccessTokenRepository.FindActiveByToken(db.DB, tokenString)
		if err != nil {
			return nil, err
		}
		if pat == nil {
			return nil, ErrInvalidBearer
		}
		touchPersonalAccessToken(pat)
		return &BearerIdentity{UserId: pat.UserId, Email: pat.Email}, nil
	}

	claims, err := services.ValidateToken(tokenString)
	if err != nil {
		return nil, ErrInvalidBearer
	}
	identity := &BearerIdentity{UserId: claims.UserId, Email: claims.Email}
	if claims.ExpiresAt != nil {
		identity.Expires = claims.ExpiresAt.Time
	}
	return identity, nil
}

func touchPersonalAccessToken(pat *transactional.ActivePAT) {
	now := time.Now()
	if pat.LastUsedAt != nil && now.Sub(*pat.LastUsedAt) < patTouchInterval {
		return
	}

	go func() {
		defer traceway.Recover()
		_, _ = db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
			return struct{}{}, transactional.PersonalAccessTokenRepository.TouchLastUsed(tx, pat.Id, now)
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
