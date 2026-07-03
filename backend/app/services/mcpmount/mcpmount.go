// Package mcpmount serves the Traceway MCP server (cli/pkg/mcpserver) over
// streamable HTTP from the backend itself. Auth follows the MCP authorization
// spec: every request must carry a bearer token (PAT, dashboard JWT, or an
// OAuth access token from the device or authorization-code flow), and a 401
// challenge points clients at the RFC 9728 protected-resource metadata so
// spec-compliant clients discover the authorization server and run the
// authorization-code + PKCE flow on their own.
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

// Path is the MCP endpoint, registered at the origin root in cmd/run.go
// (like the .well-known documents) so the resource identifier is simply
// <origin>/mcp.
const Path = "/mcp"

// patExpiryHorizon is the synthetic expiration reported for credentials with
// no intrinsic expiry (non-expiring PATs): the SDK's bearer middleware
// requires one. Validity is still re-checked on every request, so the value
// only needs to be in the future.
const patExpiryHorizon = time.Hour

// GinHandler builds the /mcp handler. Tool calls loop back into the same gin
// engine in-process, presenting each MCP request's own bearer token, so API
// authorization (project access, roles) applies to MCP exactly as it does to
// the HTTP API, and a client that rotates its access token mid-session keeps
// working.
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

// engineTransport dispatches the MCP server's API calls straight into the
// gin engine without a network round trip. The full middleware chain
// (UseAppAuth, project access) still runs on every call.
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
