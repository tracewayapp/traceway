package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/tracewayapp/traceway/backend/app/models"
)

const (
	appJWTLifetime      = 9 * time.Minute
	tokenRefreshMargin  = time.Minute
	installationTokenTT = time.Hour
)

// installationPermissions is the least an attempt needs on a repository:
// push the fix branch, open the pull request, comment, and read metadata.
var installationPermissions = map[string]string{
	"contents":      "write",
	"pull_requests": "write",
	"issues":        "write",
	"metadata":      "read",
}

type cachedToken struct {
	token   string
	expires time.Time
}

// tokenCache keeps installation tokens per integration and repository
// until a minute before they expire.
type tokenCache struct {
	mu     sync.Mutex
	tokens map[string]cachedToken
}

func newTokenCache() *tokenCache {
	return &tokenCache{tokens: map[string]cachedToken{}}
}

func (c *tokenCache) get(key string, now time.Time) (cachedToken, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.tokens[key]
	if !ok || !now.Add(tokenRefreshMargin).Before(entry.expires) {
		return cachedToken{}, false
	}
	return entry, true
}

func (c *tokenCache) put(key string, entry cachedToken) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[key] = entry
}

// appJWT signs the ten-minute RS256 token the App authenticates with.
func appJWT(s settings, now time.Time) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(s.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("the App's private key is not a PEM RSA key: %w", err)
	}
	claims := jwt.RegisteredClaims{
		Issuer:    s.AppId,
		IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(now.Add(appJWTLifetime)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
}

type installationTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// installationToken mints (or reuses) a token scoped to one repository.
func (h *Host) installationToken(ctx context.Context, integrationId int, s settings, repo *models.Repository) (string, time.Time, error) {
	if repo == nil || repo.Owner == "" || repo.Name == "" {
		return "", time.Time{}, errors.New("an installation token needs a repository")
	}
	if h.tokens == nil {
		h.tokens = newTokenCache()
	}
	key := strconv.Itoa(integrationId) + "/" + repo.Owner + "/" + repo.Name
	now := time.Now().UTC()
	if entry, ok := h.tokens.get(key, now); ok {
		return entry.token, entry.expires, nil
	}
	signed, err := appJWT(s, now)
	if err != nil {
		return "", time.Time{}, err
	}
	var minted installationTokenResponse
	body := map[string]any{"repositories": []string{repo.Name}, "permissions": installationPermissions}
	if err := h.call(ctx, signed, http.MethodPost, "/app/installations/"+s.InstallationId+"/access_tokens", body, &minted); err != nil {
		return "", time.Time{}, err
	}
	if minted.ExpiresAt.IsZero() {
		minted.ExpiresAt = now.Add(installationTokenTT)
	}
	h.tokens.put(key, cachedToken{token: minted.Token, expires: minted.ExpiresAt})
	return minted.Token, minted.ExpiresAt, nil
}
