package authserver

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
)

const userCodeAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func newOpaqueToken(prefix string) string {
	return prefix + base64.RawURLEncoding.EncodeToString(randomBytes(32))
}

func newUserCode() string {
	b := randomBytes(8)
	var sb strings.Builder
	for i, x := range b {
		if i == 4 {
			sb.WriteByte('-')
		}
		sb.WriteByte(userCodeAlphabet[int(x)%len(userCodeAlphabet)])
	}
	return sb.String()
}

func NewPATToken(scope string) string {
	if scope == models.ScopeWrite {
		return newOpaqueToken("tww_")
	}
	return newOpaqueToken("twp_")
}

func NewSetupToken() string {
	return newOpaqueToken("tws_")
}

func IssueTokenSet(tx *sql.Tx, userId int, email, clientId, scope string) (*models.TokenSetResponse, error) {
	return issueTokenSetWithFamily(tx, userId, email, clientId, scope, uuid.New().String())
}

func issueTokenSetWithFamily(tx *sql.Tx, userId int, email, clientId, scope, familyId string) (*models.TokenSetResponse, error) {
	if scope != models.ScopeWrite {
		scope = models.ScopeRead
	}
	ttl := accessTokenTTL
	access, err := services.GenerateAccessToken(userId, email, scope, clientId, ttl)
	if err != nil {
		return nil, err
	}

	refresh := newOpaqueToken("twr_")
	if err := transactional.RefreshTokenRepository.Insert(tx, refresh, familyId, userId, clientId, scope, time.Now().Add(refreshTokenTTL)); err != nil {
		return nil, err
	}

	return &models.TokenSetResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(ttl.Seconds()),
		Email:        email,
		Scope:        scope,
	}, nil
}
