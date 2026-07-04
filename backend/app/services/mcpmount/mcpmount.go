package mcpmount

import (
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
	"github.com/tracewayapp/traceway/cli/pkg/client"
	"github.com/tracewayapp/traceway/cli/pkg/mcpserver"
)

const Path = "/mcp"

const patExpiryHorizon = time.Hour

const verifiedIdentityTTL = 2 * time.Minute

func GinHandler(engine *gin.Engine, version string) gin.HandlerFunc {
	identities := &identityCache{entries: map[[32]byte]identityEntry{}}
	loopback := client.New(
		"http://traceway-mcp.internal",
		client.WithHTTPClient(&http.Client{Transport: engineTransport{engine, identities}, Timeout: 60 * time.Second}),
	)

	streamable := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpserver.New(mcpserver.Config{
			Client:           loopback,
			InstanceURL:      authserver.IssuerBaseURLFromHTTPRequest(r),
			Version:          version,
			PerRequestBearer: true,
			AuthHint:         "the access token was rejected; re-authenticate this MCP connection from your client (OAuth clients refresh automatically; PAT setups need a new token)",
		})
	}, nil)

	return func(c *gin.Context) {
		issuer := authserver.IssuerBaseURLFromRequest(c)
		requireBearer := auth.RequireBearerToken(identities.verifyBearer, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: issuer + "/.well-known/oauth-protected-resource" + Path,
		})
		requireBearer(streamable).ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

type identityCache struct {
	mu      sync.Mutex
	entries map[[32]byte]identityEntry
}

type identityEntry struct {
	identity *middleware.BearerIdentity
	expires  time.Time
}

func (c *identityCache) verifyBearer(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	identity, err := middleware.AuthenticateBearer(token)
	if err != nil {
		if errors.Is(err, middleware.ErrInvalidBearer) {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}
	c.put(token, identity)
	expires := identity.Expires
	if expires.IsZero() {
		expires = time.Now().Add(patExpiryHorizon)
	}
	return &auth.TokenInfo{
		UserID:     strconv.Itoa(identity.UserId),
		Expiration: expires,
		Extra:      map[string]any{"email": identity.Email},
	}, nil
}

func (c *identityCache) put(token string, identity *middleware.BearerIdentity) {
	now := time.Now()
	expires := now.Add(verifiedIdentityTTL)
	if !identity.Expires.IsZero() && identity.Expires.Before(expires) {
		expires = identity.Expires
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.entries {
		if now.After(entry.expires) {
			delete(c.entries, key)
		}
	}
	c.entries[sha256.Sum256([]byte(token))] = identityEntry{identity: identity, expires: expires}
}

func (c *identityCache) get(token string) *middleware.BearerIdentity {
	if token == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[sha256.Sum256([]byte(token))]
	if !ok || time.Now().After(entry.expires) {
		return nil
	}
	return entry.identity
}

type engineTransport struct {
	handler    http.Handler
	identities *identityCache
}

func (t engineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if identity := t.identities.get(bearerToken(req)); identity != nil {
		req = req.WithContext(middleware.ContextWithVerifiedBearer(req.Context(), identity))
	}
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		t.handler.ServeHTTP(rec, req)
	}()
	select {
	case <-done:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	res := rec.Result()
	res.Request = req
	return res, nil
}

func bearerToken(req *http.Request) string {
	fields := strings.Fields(req.Header.Get("Authorization"))
	if len(fields) == 2 && strings.EqualFold(fields[0], "bearer") {
		return fields[1]
	}
	return ""
}
