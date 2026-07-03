package authserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/tracewayapp/traceway/backend/app/config"
)

const (
	accessTokenTTL     = 15 * time.Minute
	refreshTokenTTL    = 90 * 24 * time.Hour
	deviceCodeTTL      = 10 * time.Minute
	devicePollInterval = 5 * time.Second
	authCodeTTL        = 5 * time.Minute
	cliClientID        = "traceway-cli"
	mcpClientID        = "traceway-mcp"

	// reuseGrace is the window during which re-presenting a just-used refresh
	// token is treated as a benign concurrent retry (two CLI processes racing,
	// or a client that lost the rotated response) rather than a stolen-token
	// replay: within it the family is NOT revoked. Genuine replays surface long
	// after this window, once the legitimate client has moved on.
	reuseGrace = 30 * time.Second
)

// IssuerBaseURL is the OAuth issuer and verification-URL base. It is always the
// server we are running on. Prefer IssuerBaseURLFromRequest in request handlers
// so the issuer resolves even when APP_BASE_URL is unset.
func IssuerBaseURL() string {
	return strings.TrimRight(config.Config.AppBaseURL, "/")
}

// IssuerBaseURLFromRequest returns the configured issuer base URL, or derives it
// from the incoming request when APP_BASE_URL is unset — the common case on
// self-hosted deployments that never set it. It honors X-Forwarded-Proto /
// X-Forwarded-Host from a fronting reverse proxy, falling back to the request's
// own scheme and Host. The returned value has no trailing slash.
func IssuerBaseURLFromRequest(c *gin.Context) string {
	if c.Request == nil {
		return IssuerBaseURL()
	}
	return IssuerBaseURLFromHTTPRequest(c.Request)
}

// IssuerBaseURLFromHTTPRequest is IssuerBaseURLFromRequest for handlers that
// run outside gin (the MCP mount's streamable HTTP handler).
func IssuerBaseURLFromHTTPRequest(r *http.Request) string {
	if base := IssuerBaseURL(); base != "" {
		return base
	}

	scheme := "http"
	if proto := firstForwardedValue(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = proto
	} else if r.TLS != nil {
		scheme = "https"
	}

	host := firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// firstForwardedValue returns the first entry of a possibly comma-separated
// X-Forwarded-* header value (proxies append, so the client-facing value is
// first), trimmed of whitespace.
func firstForwardedValue(header string) string {
	if header == "" {
		return ""
	}
	if i := strings.IndexByte(header, ','); i >= 0 {
		header = header[:i]
	}
	return strings.TrimSpace(header)
}
