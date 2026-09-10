package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/agent"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

// memberIdentityController lets an organization admin bind a member to an
// external account the agent's surfaces authorize through (a GitHub login);
// surfaces that can match on email, like Slack, fill identities themselves.
type memberIdentityController struct{}

type identityView struct {
	Provider   string `json:"provider"`
	ExternalId string `json:"externalId"`
	Display    string `json:"display"`
}

type identityRequest struct {
	ExternalId string `json:"externalId"`
	Display    string `json:"display"`
}

var identityProviders = map[string]bool{"github": true, "slack": true}

func memberParams(ctx *gin.Context) (int, int, bool) {
	organizationId := ctx.GetInt(middleware.OrganizationIdContextKey)
	userId, err := strconv.Atoi(ctx.Param("userId"))
	if err != nil || organizationId == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid member"})
		return 0, 0, false
	}
	isMember, err := transactional.OrganizationRepository.IsUserMember(db.GetTx(ctx), organizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check membership: %w", err))
		return 0, 0, false
	}
	if !isMember {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Member not found"})
		return 0, 0, false
	}
	return organizationId, userId, true
}

func (ctrl *memberIdentityController) List(ctx *gin.Context) {
	_, userId, ok := memberParams(ctx)
	if !ok {
		return
	}
	rows, err := transactional.IdentityRepository.FindByUser(db.GetTx(ctx), userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to list identities: %w", err))
		return
	}
	out := make([]identityView, 0, len(rows))
	for _, row := range rows {
		out = append(out, identityView{Provider: row.Provider, ExternalId: row.ExternalId, Display: row.Display})
	}
	ctx.JSON(http.StatusOK, gin.H{"identities": out})
}

// Set binds the member to one external account of a provider, replacing
// the previous binding of that provider. External ids are stored lowercase
// because GitHub logins and Slack ids compare case-insensitively.
func (ctrl *memberIdentityController) Set(ctx *gin.Context) {
	_, userId, ok := memberParams(ctx)
	if !ok {
		return
	}
	provider := ctx.Param("provider")
	if !identityProviders[provider] {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Unknown identity provider."})
		return
	}
	var req identityRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	externalId := strings.ToLower(strings.TrimSpace(req.ExternalId))
	if externalId == "" || len(externalId) > 200 || strings.ContainsAny(externalId, " \t\n@") {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Enter the account's login without spaces or @."})
		return
	}
	display := strings.TrimSpace(req.Display)
	if display == "" {
		display = strings.TrimSpace(req.ExternalId)
	}
	tx := db.GetTx(ctx)
	taken, err := transactional.IdentityRepository.FindByExternalId(tx, provider, externalId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to look up identity: %w", err))
		return
	}
	if taken != nil && taken.UserId != userId {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "That account is already linked to another member."})
		return
	}
	existing, err := transactional.IdentityRepository.FindByUserAndProvider(tx, userId, provider)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load identity: %w", err))
		return
	}
	if existing != nil {
		if err := transactional.IdentityRepository.Delete(tx, existing.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to replace identity: %w", err))
			return
		}
	}
	row := &models.Identity{UserId: userId, Provider: provider, ExternalId: externalId, Display: display, CreatedAt: time.Now().UTC()}
	if _, err := transactional.IdentityRepository.Create(tx, row); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create identity: %w", err))
		return
	}
	middleware.OnCommit(ctx, agent.IntegrationsChanged)
	ctx.JSON(http.StatusOK, gin.H{"identity": identityView{Provider: provider, ExternalId: externalId, Display: display}})
}

func (ctrl *memberIdentityController) Clear(ctx *gin.Context) {
	_, userId, ok := memberParams(ctx)
	if !ok {
		return
	}
	tx := db.GetTx(ctx)
	existing, err := transactional.IdentityRepository.FindByUserAndProvider(tx, userId, ctx.Param("provider"))
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load identity: %w", err))
		return
	}
	if existing != nil {
		if err := transactional.IdentityRepository.Delete(tx, existing.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete identity: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": existing != nil})
}

var MemberIdentityController = memberIdentityController{}
