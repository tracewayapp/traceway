package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

type wellKnownController struct{}

func (c *wellKnownController) AuthorizationServer(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	// response_types_supported is REQUIRED by RFC 8414 even though neither
	// supported grant uses the authorization endpoint; "none" keeps spec-strict
	// clients (e.g. the MCP SDK's metadata schema) from rejecting the document.
	ctx.JSON(http.StatusOK, gin.H{
		"issuer":                                issuer,
		"token_endpoint":                        issuer + "/api/auth/token",
		"device_authorization_endpoint":         issuer + "/api/auth/device/authorize",
		"grant_types_supported":                 []string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
		"response_types_supported":              []string{"none"},
		"token_endpoint_auth_methods_supported": []string{"none"},
	})
}

func (c *wellKnownController) ProtectedResource(ctx *gin.Context) {
	issuer := authserver.IssuerBaseURLFromRequest(ctx)
	ctx.JSON(http.StatusOK, gin.H{
		"resource":              issuer,
		"authorization_servers": []string{issuer},
	})
}

var WellKnownController = wellKnownController{}
