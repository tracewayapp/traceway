package mcpmount

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func GinHandler(engine *gin.Engine, version string) gin.HandlerFunc {
	loopback := client.New(
		"http://traceway-mcp.internal",
		client.WithHTTPClient(&http.Client{Transport: engineTransport{engine}, Timeout: 60 * time.Second}),
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
		requireBearer := auth.RequireBearerToken(verifyBearer, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: issuer + "/.well-known/oauth-protected-resource",
		})
		requireBearer(streamable).ServeHTTP(c.Writer, c.Request)
		c.Abort()
	}
}

func verifyBearer(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	identity, err := middleware.AuthenticateBearer(token)
	if err != nil {
		if errors.Is(err, middleware.ErrInvalidBearer) {
			return nil, auth.ErrInvalidToken
		}
		return nil, err
	}
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

type engineTransport struct {
	handler http.Handler
}

func (t engineTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	t.handler.ServeHTTP(rec, req)
	res := rec.Result()
	res.Request = req
	return res, nil
}
