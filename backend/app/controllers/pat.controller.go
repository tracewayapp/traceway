package controllers

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services/authserver"
)

type patController struct{}

func (c *patController) Create(ctx *gin.Context) {
	tx := db.GetTx(ctx)
	userId := middleware.GetUserId(ctx)

	var req models.CreatePATRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name is required"})
		return
	}
	if utf8.RuneCountInString(name) > 100 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Name must be 100 characters or fewer"})
		return
	}
	if req.ExpiresInDays != nil && (*req.ExpiresInDays < 1 || *req.ExpiresInDays > 3650) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Expiry must be between 1 and 3650 days"})
		return
	}

	scope, ok := parsePATScope(req.Scope)
	if !ok {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Access must be read or write"})
		return
	}
	if scope == models.ScopeWrite && (req.ExpiresInDays == nil || *req.ExpiresInDays < 1 || *req.ExpiresInDays > maxWritePATDays) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Write tokens must expire within 90 days"})
		return
	}

	token := authserver.NewPATToken(scope)
	id := uuid.New().String()
	prefix := patPrefix(token)

	var expiresAt *time.Time
	if req.ExpiresInDays != nil && *req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInDays) * 24 * time.Hour).UTC()
		expiresAt = &t
	}

	if err := transactional.PersonalAccessTokenRepository.Create(tx, id, token, prefix, userId, name, scope, expiresAt); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("create pat: %w", err))
		return
	}

	ctx.JSON(http.StatusCreated, models.CreatePATResponse{
		Id:        id,
		Name:      name,
		Prefix:    prefix,
		Token:     token,
		Scope:     scope,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: expiresAt,
	})
}

func (c *patController) List(ctx *gin.Context) {
	userId := middleware.GetUserId(ctx)

	tokens, err := transactional.PersonalAccessTokenRepository.ListByUser(db.DB, userId)
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

	rows, err := transactional.PersonalAccessTokenRepository.Revoke(tx, id, userId)
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

const maxWritePATDays = 90

func parsePATScope(raw string) (string, bool) {
	switch strings.TrimSpace(raw) {
	case "", "read", models.ScopeRead:
		return models.ScopeRead, true
	case "write", models.ScopeWrite:
		return models.ScopeWrite, true
	}
	return "", false
}

func patPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

var PATController = patController{}
