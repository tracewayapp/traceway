package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

type patController struct{}

func (c *patController) Create(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	var req models.CreatePATRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required"})
		return
	}

	token := authserver.NewPATToken()
	id := uuid.New().String()
	prefix := patPrefix(token)

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour).UTC()
		expiresAt = &t
	}

	if err := repositories.PersonalAccessTokenRepository.Create(tx, id, token, prefix, userId, strings.TrimSpace(req.Name), expiresAt); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("create pat: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, models.CreatePATResponse{
		Id:        id,
		Name:      strings.TrimSpace(req.Name),
		Prefix:    prefix,
		Token:     token,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	})
}

func (c *patController) List(ctx *gin.Context) {
	userId := middleware.GetUserId(ctx)

	tokens, err := repositories.PersonalAccessTokenRepository.ListByUser(db.DB, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("list pats: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, tokens)
}

func (c *patController) Revoke(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)
	id := ctx.Param("id")

	rows, err := repositories.PersonalAccessTokenRepository.Revoke(tx, id, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("revoke pat: %w", err))
		return
	}
	if rows == 0 {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

func patPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

var PATController = patController{}
