package middleware

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
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
		// Every dashboard route binds JSON that encoding/json buffers whole, so
		// the body is capped once here rather than at each bind site.
		if c.Request.Body != nil && c.Request.Body != http.NoBody {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTransactionalBodyBytes)
		}
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
		c.Set(BearerIdentityContextKey, identity)

		if identity.IsAgent() {
			authorizeAgentRequest(c, identity)
			return
		}

		c.Next()
	}
}

var ErrInvalidBearer = errors.New("invalid bearer token")

const BearerIdentityContextKey = "bearerIdentity"

const (
	CredentialSession = "session"
	CredentialPAT     = "pat"
	CredentialOAuth   = "oauth"
)

// legacyAgentTokenLifetime separates the access tokens minted before scopes
// existed (15 minutes, device and OAuth grants) from dashboard sessions (7
// days): an unscoped JWT this short-lived came from the CLI or an MCP client
// and is held to read access instead of being trusted as a session.
const legacyAgentTokenLifetime = time.Hour

type BearerIdentity struct {
	UserId         int
	Email          string
	Expires        time.Time
	Scope          string
	CredentialKind string
	CredentialRef  string
}

func (b *BearerIdentity) IsAgent() bool {
	return b.CredentialKind != CredentialSession
}

// HasAgentWriteScope reports whether this is an agent credential carrying the
// write scope. A dashboard session never qualifies: MCP is an agent surface,
// and a session behind /mcp/write would skip the agent gate's organization
// opt-in and audit log, since sessions bypass the gate.
func (b *BearerIdentity) HasAgentWriteScope() bool {
	return b.IsAgent() && b.Scope == models.ScopeWrite
}

func AuthenticateBearer(tokenString string) (*BearerIdentity, error) {
	if strings.HasPrefix(tokenString, "twp_") || strings.HasPrefix(tokenString, "tww_") {
		pat, err := transactional.PersonalAccessTokenRepository.FindActiveByToken(db.DB, tokenString)
		if err != nil {
			return nil, err
		}
		if pat == nil {
			return nil, ErrInvalidBearer
		}
		touchPersonalAccessToken(pat)
		return &BearerIdentity{UserId: pat.UserId, Email: pat.Email, Scope: pat.Scope, CredentialKind: CredentialPAT, CredentialRef: pat.Id}, nil
	}

	claims, err := services.ValidateToken(tokenString)
	if err != nil {
		return nil, ErrInvalidBearer
	}
	identity := &BearerIdentity{UserId: claims.UserId, Email: claims.Email, CredentialKind: CredentialSession}
	if claims.ExpiresAt != nil {
		identity.Expires = claims.ExpiresAt.Time
	}
	switch {
	case claims.Scope != "":
		identity.Scope = claims.Scope
		identity.CredentialKind = CredentialOAuth
		identity.CredentialRef = claims.ClientId
	case claims.ExpiresAt != nil && claims.IssuedAt != nil && claims.ExpiresAt.Sub(claims.IssuedAt.Time) <= legacyAgentTokenLifetime:
		identity.Scope = models.ScopeRead
		identity.CredentialKind = CredentialOAuth
	}
	return identity, nil
}

func GetBearerIdentity(c *gin.Context) *BearerIdentity {
	if v, ok := c.Get(BearerIdentityContextKey); ok {
		if identity, ok := v.(*BearerIdentity); ok {
			return identity
		}
	}
	return nil
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
