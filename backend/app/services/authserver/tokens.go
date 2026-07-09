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

func NewPATToken() string {
	return newOpaqueToken("twp_")
}

func IssueTokenSet(tx *sql.Tx, userId int, email, clientId string) (*models.TokenSetResponse, error) {
	return issueTokenSetWithFamily(tx, userId, email, clientId, uuid.New().String())
}

func issueTokenSetWithFamily(tx *sql.Tx, userId int, email, clientId, familyId string) (*models.TokenSetResponse, error) {
	ttl := accessTokenTTL
	access, err := services.GenerateAccessToken(userId, email, ttl)
	if err != nil {
		return nil, err
	}

	refresh := newOpaqueToken("twr_")
	if err := transactional.RefreshTokenRepository.Insert(tx, refresh, familyId, userId, clientId, time.Now().Add(refreshTokenTTL)); err != nil {
		return nil, err
	}

	return &models.TokenSetResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    "Bearer",
		ExpiresIn:    int(ttl.Seconds()),
		Email:        email,
	}, nil
}
