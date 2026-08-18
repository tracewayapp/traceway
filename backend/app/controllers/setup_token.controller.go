package controllers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

const setupTokenTTL = 60 * time.Minute

type setupTokenController struct{}

func (s setupTokenController) Create(ctx *gin.Context) {
	var req models.CreateSetupTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	if !requireOrgWrite(ctx, tx, req.OrganizationId) {
		return
	}

	if err := transactional.SetupTokenRepository.DeleteByUserAndOrganization(tx, userId, req.OrganizationId); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("replace setup token: %w", err))
		return
	}

	token := authserver.NewSetupToken()
	expiresAt := time.Now().Add(setupTokenTTL).UTC()

	if err := transactional.SetupTokenRepository.Create(tx, uuid.New().String(), token, patPrefix(token), userId, req.OrganizationId, expiresAt); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("create setup token: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, models.CreateSetupTokenResponse{
		Token:          token,
		Prefix:         patPrefix(token),
		ExpiresAt:      expiresAt,
		OrganizationId: req.OrganizationId,
		BackendUrl:     models.GetBackendUrl(),
	})
}

var SetupTokenController = setupTokenController{}
