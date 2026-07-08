package controllers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

type oauthAuthorizeController struct{}

func (c *oauthAuthorizeController) Register(ctx *gin.Context) {
	setTokenResponseHeaders(ctx)

	var req models.RegisterClientRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client_metadata", "error_description": "body must be a JSON object with client_name and redirect_uris"})
		return
	}

	client, err := authserver.RegisterClient(db.GetTx(ctx), req.ClientName, req.RedirectUris)
	if err != nil {
		if errors.Is(err, authserver.ErrInvalidClientMetadata) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"error":             "invalid_client_metadata",
				"error_description": strings.TrimPrefix(err.Error(), "invalid_client_metadata: "),
			})
			return
		}
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("oauth register: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, models.RegisterClientResponse{
		ClientId:                client.Id,
		ClientName:              client.Name,
		RedirectUris:            client.RedirectUris,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		ClientIdIssuedAt:        client.CreatedAt.Unix(),
	})
}

func (c *oauthAuthorizeController) Lookup(ctx *gin.Context) {
	clientId := ctx.Query("client_id")
	if clientId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "client_id is required"})
		return
	}

	client, err := repositories.OauthClientRepository.FindById(db.DB, clientId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("oauth client lookup: %w", err))
		return
	}
	if client == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	ctx.JSON(http.StatusOK, models.OauthClientLookupResponse{ClientName: client.Name})
}

func (c *oauthAuthorizeController) Approve(ctx *gin.Context) {
	var req models.AuthorizeApproveRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	redirectTo, err := authserver.ApproveAuthorization(db.GetTx(ctx), middleware.GetUserId(ctx), req, authserver.IssuerBaseURLFromRequest(ctx))
	if err != nil {
		writeAuthorizeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, models.AuthorizeRedirectResponse{RedirectTo: redirectTo})
}

func (c *oauthAuthorizeController) Deny(ctx *gin.Context) {
	var req models.AuthorizeDenyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
		return
	}

	redirectTo, err := authserver.DenyAuthorization(db.DB, req.ClientId, req.RedirectUri, req.State)
	if err != nil {
		writeAuthorizeError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, models.AuthorizeRedirectResponse{RedirectTo: redirectTo})
}

func writeAuthorizeError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, authserver.ErrUnknownClient):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_client"})
	case errors.Is(err, authserver.ErrInvalidRedirect):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_redirect_uri"})
	case errors.Is(err, authserver.ErrInvalidChallenge), errors.Is(err, authserver.ErrInvalidGrantRequest):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_request"})
	case errors.Is(err, authserver.ErrInvalidTarget):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid_target"})
	default:
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("oauth authorize: %w", err))
	}
}

var OauthAuthorizeController = oauthAuthorizeController{}
