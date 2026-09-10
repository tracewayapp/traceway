package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/integrations/github"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	traceway "go.tracewayapp.com"
)

// githubManifestController is the GitHub App manifest flow: an
// authenticated start that signs the state, an anonymous page that submits
// the manifest to GitHub, and GitHub's callback that stores the App's
// credentials and sends the browser on to install it.
type githubManifestController struct {
	host *github.Host
}

// GitHubHost is the provider the manifest routes act on; cmd/run.go sets
// it to the registered one.
var GitHubHost *github.Host

type manifestStartRequest struct {
	OrganizationId int `json:"organizationId"`
	IntegrationId  int `json:"integrationId"`
}

// Start signs the state for an organization admin and returns the page to
// open. An existing github integration may be named so the App lands on it.
func (ctrl *githubManifestController) Start(ctx *gin.Context) {
	userId := middleware.GetUserId(ctx)
	if userId == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in to create the GitHub App."})
		return
	}
	var req manifestStartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil || req.OrganizationId == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "organizationId is required"})
		return
	}
	tx := db.GetTx(ctx)
	role, err := transactional.OrganizationRepository.GetUserRole(tx, req.OrganizationId, userId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to check organization role: %w", err))
		return
	}
	if role != "owner" && role != "admin" {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only organization admins can create the GitHub App."})
		return
	}
	if req.IntegrationId != 0 {
		in, err := transactional.IntegrationRepository.FindById(tx, req.IntegrationId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load integration: %w", err))
			return
		}
		if in == nil || in.OrganizationId != req.OrganizationId || in.Provider != github.Provider {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Pick a GitHub integration of this organization."})
			return
		}
	}
	start, err := github.StartManifest(config.Config.PublicBaseURLOrDev(), req.OrganizationId, req.IntegrationId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to sign the manifest state: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, start)
}

// Page renders the self-submitting manifest form; the state was signed by
// Start, so the page itself needs no session.
func (ctrl *githubManifestController) Page(ctx *gin.Context) {
	page, err := github.ManifestPage(config.Config.PublicBaseURLOrDev(), ctx.Query("state"))
	if err != nil {
		ctx.String(http.StatusBadRequest, "The manifest link is invalid or has expired. Start again from Settings.")
		return
	}
	ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
}

// Callback is where GitHub sends the browser after the App is created.
func (ctrl *githubManifestController) Callback(ctx *gin.Context) {
	if ctrl.host == nil {
		ctrl.host = GitHubHost
	}
	if ctrl.host == nil {
		ctx.String(http.StatusServiceUnavailable, "GitHub integrations are not registered on this instance.")
		return
	}
	installURL, err := ctrl.host.CompleteManifest(ctx.Request.Context(), ctx.Query("code"), ctx.Query("state"), 0)
	if err != nil {
		traceway.CaptureException(traceway.NewStackTraceErrorf("github manifest callback failed: %w", err))
		ctx.String(http.StatusBadRequest, "GitHub did not complete the App creation: "+err.Error())
		return
	}
	ctx.Redirect(http.StatusFound, installURL)
}

var GitHubManifestController = githubManifestController{}
