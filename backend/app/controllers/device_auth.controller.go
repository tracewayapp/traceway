package controllers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

type deviceAuthController struct{}

func (c *deviceAuthController) Authorize(ctx *gin.Context) {
	setTokenResponseHeaders(ctx)

	var req models.DeviceAuthorizeRequest
	// ShouldBind honors both JSON (the CLI) and form encoding (RFC 8628 §3.4).
	// client_id is optional, so a malformed/absent body just falls through to the
	// default client — CreateDeviceAuth substitutes it — rather than 400ing.
	_ = ctx.ShouldBind(&req)

	resp, err := authserver.CreateDeviceAuth(req.ClientId, authserver.IssuerBaseURLFromRequest(ctx))
	if err != nil {
		if errors.Is(err, authserver.ErrUnknownClient) {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client"})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("device authorize: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

func (c *deviceAuthController) Token(ctx *gin.Context) {
	setTokenResponseHeaders(ctx)

	var req models.TokenRequest
	// ShouldBind honors both JSON (the CLI) and application/x-www-form-urlencoded
	// (RFC 6749 §3.2 / RFC 8628 §3.4, which standard OAuth clients send).
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	switch req.GrantType {
	case "device_code", "urn:ietf:params:oauth:grant-type:device_code":
		ts, err := authserver.PollDeviceToken(req.DeviceCode)
		if err != nil {
			writeOAuthGrantError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, ts)
	case "refresh_token":
		ts, err := authserver.RotateRefresh(req.RefreshToken)
		if err != nil {
			writeOAuthGrantError(ctx, err)
			return
		}
		ctx.JSON(http.StatusOK, ts)
	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
	}
}

// Logout revokes the entire refresh-token family the presented token belongs
// to, so `traceway logout` actually invalidates the session server-side. It is
// intentionally unauthenticated (the refresh token is the credential) and
// always returns 200 — it never reveals whether the token existed.
func (c *deviceAuthController) Logout(ctx *gin.Context) {
	var req models.TokenRequest
	_ = ctx.ShouldBind(&req)

	if req.RefreshToken != "" {
		if err := authserver.RevokeByToken(req.RefreshToken); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("logout revoke: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *deviceAuthController) Lookup(ctx *gin.Context) {
	userCode := ctx.Query("user_code")
	if userCode == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "user_code is required"})
		return
	}

	da, err := repositories.DeviceAuthorizationRepository.FindByUserCode(db.DB, userCode)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("device lookup: %w", err))
		return
	}
	if da == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if time.Now().After(da.ExpiresAt) {
		ctx.JSON(http.StatusGone, gin.H{"error": "expired"})
		return
	}

	ctx.JSON(http.StatusOK, models.DeviceLookupResponse{
		UserCode:    da.UserCode,
		Status:      da.Status,
		ClientName:  clientDisplayName(da.ClientId),
		RequestedAt: da.CreatedAt,
		ExpiresAt:   da.ExpiresAt,
	})
}

func (c *deviceAuthController) Approve(ctx *gin.Context) {
	c.resolve(ctx, true)
}

func (c *deviceAuthController) Deny(ctx *gin.Context) {
	c.resolve(ctx, false)
}

func (c *deviceAuthController) resolve(ctx *gin.Context, approve bool) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	var req models.DeviceApproveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.UserCode == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "user_code is required"})
		return
	}

	var err error
	if approve {
		err = authserver.Approve(tx, req.UserCode, userId)
	} else {
		err = authserver.Deny(tx, req.UserCode)
	}
	if err != nil {
		if errors.Is(err, authserver.ErrDeviceNotFound) {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		if errors.Is(err, authserver.ErrDeviceExpired) {
			ctx.JSON(http.StatusGone, gin.H{"error": "expired"})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("device resolve: %w", err))
		return
	}

	status := "approved"
	if !approve {
		status = "denied"
	}
	ctx.JSON(http.StatusOK, gin.H{"status": status})
}

func writeOAuthGrantError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, authserver.ErrAuthorizationPending):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "authorization_pending"})
	case errors.Is(err, authserver.ErrSlowDown):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "slow_down"})
	case errors.Is(err, authserver.ErrAccessDenied):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "access_denied"})
	case errors.Is(err, authserver.ErrExpiredToken):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "expired_token"})
	case errors.Is(err, authserver.ErrInvalidGrant):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_grant"})
	default:
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("token grant: %w", err))
	}
}

// setTokenResponseHeaders marks OAuth responses uncacheable, as RFC 6749 §5.1
// and RFC 8628 §3.2 require for responses that carry tokens or device codes.
// It applies to error responses from the same endpoints too.
func setTokenResponseHeaders(ctx *gin.Context) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
}

// clientDisplayName never echoes an unrecognized client_id: the approval page
// renders this as the trusted requester name.
func clientDisplayName(clientId string) string {
	switch clientId {
	case "traceway-cli", "":
		return "Traceway CLI"
	case "traceway-mcp":
		return "Traceway MCP"
	default:
		return "An unrecognized application"
	}
}

var DeviceAuthController = deviceAuthController{}
