package middleware

import (
	"database/sql"
	"errors"
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

// ErrInvalidBearer marks a well-formed but unacceptable credential (unknown
// PAT, bad or expired JWT). Anything else out of AuthenticateBearer is a
// server-side failure.
var ErrInvalidBearer = errors.New("invalid bearer token")

// BearerIdentity is the resolved principal behind a bearer credential.
// Expires is zero when the credential has no intrinsic expiry (a
// non-expiring PAT).
type BearerIdentity struct {
	UserId  int
	Email   string
	Expires time.Time
}

// AuthenticateBearer resolves a bearer credential (PAT or JWT) to its user.
// It is the shared core of UseAppAuth and the MCP mount's token verifier.
func AuthenticateBearer(tokenString string) (*BearerIdentity, error) {
	if strings.HasPrefix(tokenString, "twp_") {
		pat, err := repositories.PersonalAccessTokenRepository.FindActiveByToken(db.DB, tokenString)
		if err != nil {
			return nil, err
		}
		if pat == nil {
			return nil, ErrInvalidBearer
		}
		touchPersonalAccessToken(pat)
		// FindActiveByToken already rejects expired PATs; the row's own expiry
		// (if any) is not surfaced, so Expires stays zero and callers that
		// need a horizon (the MCP mount) synthesize a short one.
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
