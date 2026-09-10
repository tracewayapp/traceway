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
	"github.com/google/uuid"
)

const UserIdContextKey = "userId"
const UserEmailContextKey = "userEmail"

// RunAttemptIdContextKey and RunProjectIdContextKey are set instead of a
// user when the bearer is an agent run token: a readonly credential scoped
// to one attempt's project, with no user behind it.
const RunAttemptIdContextKey = "runAttemptId"
const RunProjectIdContextKey = "runProjectId"

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

		if identity.RunAttemptId != uuid.Nil {
			c.Set(RunAttemptIdContextKey, identity.RunAttemptId)
			c.Set(RunProjectIdContextKey, identity.RunProjectId)
			c.Next()
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
	// RunAttemptId and RunProjectId are set for an agent run token, which
	// carries no user: it reads one project as readonly for a short while.
	RunAttemptId uuid.UUID
	RunProjectId uuid.UUID
}

func AuthenticateBearer(tokenString string) (*BearerIdentity, error) {
	// Any token that is not a valid run token falls through to the user
	// shapes, which give the accurate rejection for a PAT, an expired
	// session or a malformed string.
	if run, err := services.ValidateRunToken(tokenString); err == nil {
		return &BearerIdentity{
			RunAttemptId: uuid.MustParse(run.AttemptId),
			RunProjectId: uuid.MustParse(run.ProjectId),
			Expires:      run.ExpiresAt.Time,
		}, nil
	}
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

// RunAttemptId returns the attempt a run token is scoped to, or uuid.Nil
// when the caller is a user.
func RunAttemptId(c *gin.Context) uuid.UUID {
	if id, exists := c.Get(RunAttemptIdContextKey); exists {
		return id.(uuid.UUID)
	}
	return uuid.Nil
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
