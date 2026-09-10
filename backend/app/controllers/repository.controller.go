package controllers

import (
	"net/http"
	"regexp"
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

type repositoryController struct{}

var repositoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// Get returns the project's repository binding, or null, together with the
// organization's code host integrations the binding can pick from.
func (ctrl *repositoryController) Get(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	repository, err := transactional.RepositoryRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load repository: %w", err))
		return
	}
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil || project == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	hosts := []codeHostView{}
	if project.OrganizationId != nil {
		integrations, err := transactional.IntegrationRepository.FindByOrganization(tx, *project.OrganizationId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load integrations: %w", err))
			return
		}
		for _, integration := range integrations {
			if integration.Enabled && hasKind(integration.Kinds, agent.KindCodeHost) {
				hosts = append(hosts, codeHostView{Id: integration.Id, Name: integration.Name, Provider: integration.Provider, Status: describeIntegration(integration)})
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"repository": repository, "codeHosts": hosts})
}

type upsertRepositoryRequest struct {
	IntegrationId *int   `json:"integrationId"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	DefaultBranch string `json:"defaultBranch"`
	Image         string `json:"image"`
	SetupCommand  string `json:"setupCommand"`
	TestCommand   string `json:"testCommand"`
}

// Upsert creates or replaces the project's one repository binding.
func (ctrl *repositoryController) Upsert(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	var req upsertRepositoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		middleware.RejectBindError(ctx, err, "Invalid request body")
		return
	}
	req.Owner, req.Name = strings.TrimSpace(req.Owner), strings.TrimSpace(req.Name)
	req.DefaultBranch = strings.TrimSpace(req.DefaultBranch)
	if req.DefaultBranch == "" {
		req.DefaultBranch = "main"
	}
	if !repositoryNamePattern.MatchString(req.Owner) || !repositoryNamePattern.MatchString(req.Name) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Owner and repository name may only contain letters, digits, dots, dashes and underscores."})
		return
	}
	if len(req.DefaultBranch) > 200 || strings.ContainsAny(req.DefaultBranch, " \t\n") {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The default branch is not a valid git ref."})
		return
	}
	if len(req.Image) > 500 || len(req.SetupCommand) > 4000 || len(req.TestCommand) > 4000 {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Image or command is too long."})
		return
	}

	tx := db.GetTx(ctx)
	project, err := transactional.ProjectRepository.FindById(tx, projectId)
	if err != nil || project == nil || project.OrganizationId == nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load project: %w", err))
		return
	}
	if req.IntegrationId != nil {
		integration, err := transactional.IntegrationRepository.FindById(tx, *req.IntegrationId)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load integration: %w", err))
			return
		}
		if integration == nil || integration.OrganizationId != *project.OrganizationId || !hasKind(integration.Kinds, agent.KindCodeHost) {
			ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Pick a code host integration of this organization."})
			return
		}
	}

	now := time.Now().UTC()
	existing, err := transactional.RepositoryRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load repository: %w", err))
		return
	}
	repository := existing
	if repository == nil {
		repository = &models.Repository{ProjectId: projectId, CreatedAt: now}
	}
	repository.IntegrationId = req.IntegrationId
	repository.Owner = req.Owner
	repository.Name = req.Name
	repository.DefaultBranch = req.DefaultBranch
	repository.Image = strings.TrimSpace(req.Image)
	repository.SetupCommand = strings.TrimSpace(req.SetupCommand)
	repository.TestCommand = strings.TrimSpace(req.TestCommand)
	repository.UpdatedAt = now
	if existing == nil {
		id, err := transactional.RepositoryRepository.Create(tx, repository)
		if err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to create repository: %w", err))
			return
		}
		repository.Id = id
		ctx.JSON(http.StatusCreated, gin.H{"repository": repository})
		return
	}
	if err := transactional.RepositoryRepository.Update(tx, repository); err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to update repository: %w", err))
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"repository": repository})
}

func (ctrl *repositoryController) Delete(ctx *gin.Context) {
	projectId, err := middleware.GetProjectId(ctx)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	tx := db.GetTx(ctx)
	existing, err := transactional.RepositoryRepository.FindByProject(tx, projectId)
	if err != nil {
		ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load repository: %w", err))
		return
	}
	if existing != nil {
		if err := transactional.RepositoryRepository.Delete(tx, existing.Id); err != nil {
			ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to delete repository: %w", err))
			return
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"deleted": true})
}

var RepositoryController = repositoryController{}

// codeHostView is what the repository tab picks from: no config, plus the
// provider's one-line state when it has one.
type codeHostView struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Status   string `json:"status,omitempty"`
}

func describeIntegration(in *models.Integration) string {
	host, ok := agent.CodeHostFor(in.Provider)
	if !ok {
		return ""
	}
	if describer, ok := host.(agent.Describer); ok {
		return describer.Describe(in)
	}
	return ""
}
