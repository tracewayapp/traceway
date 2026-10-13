package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
	"github.com/tracewayapp/traceway/backend/app/services/mcpmount"
)

type wellKnownController struct{}

func (c *wellKnownController) AuthorizationServer(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	ctx.JSON(http.StatusOK, gin.H{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/api/auth/token",
		"device_authorization_endpoint":         issuer + "/api/auth/device/authorize",
		"registration_endpoint":                 issuer + "/api/oauth/register",
		"grant_types_supported":                 []string{"authorization_code", "urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"response_types_supported":              []string{"code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{models.ScopeRead, models.ScopeWrite},
	})
}

func (c *wellKnownController) ProtectedResource(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	ctx.JSON(http.StatusOK, gin.H{
		"resource":              issuer,
		"authorization_servers": []string{issuer},
		"scopes_supported":      []string{models.ScopeRead, models.ScopeWrite},
	})
}

func (c *wellKnownController) ProtectedResourceMCP(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	ctx.JSON(http.StatusOK, gin.H{
		"resource":              issuer + mcpmount.Path,
		"authorization_servers": []string{issuer},
		"scopes_supported":      []string{models.ScopeRead},
	})
}

func (c *wellKnownController) ProtectedResourceMCPWrite(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	ctx.JSON(http.StatusOK, gin.H{
		"resource":              issuer + mcpmount.WritePath,
		"authorization_servers": []string{issuer},
		"scopes_supported":      []string{models.ScopeWrite},
	})
}

var WellKnownController = wellKnownController{}
