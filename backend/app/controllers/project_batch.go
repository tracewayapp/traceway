package controllers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const maxBatchProjects = 20

type BatchProjectInput struct {
	Name      string `json:"name"`
	Framework string `json:"framework"`
}

type BatchProjectResult struct {
	Project *models.Project
	Status  string // "created" | "existing"
}

type batchValidationError struct{ Message string }

func (e *batchValidationError) Error() string { return e.Message }

func validateProjectName(name string) string {
	nameLen := utf8.RuneCountInString(name)
	if nameLen < 1 || nameLen > 100 {
		return "Project name must be between 1 and 100 characters"
	}
	if !projectNameRegex.MatchString(name) {
		return "Project name can only contain letters, numbers, spaces, hyphens, and underscores"
	}
	return ""
}

const invalidFrameworkMessage = "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, laravel, django, flutter, android, ios"

// batchCreateProjects validates and creates inputs in orgId inside tx.
// Idempotent by trimmed exact-match name within the org: an existing project
// with the same name is returned with Status "existing" instead of erroring,
// so a retried batch never duplicates. The whole batch is atomic: any
// validation or limit failure rolls everything back.
func batchCreateProjects(tx *sql.Tx, orgId int, createdBy int, inputs []BatchProjectInput) ([]BatchProjectResult, error) {
	if len(inputs) == 0 {
		return nil, &batchValidationError{"At least one project is required"}
	}
	if len(inputs) > maxBatchProjects {
		return nil, &batchValidationError{"At most 20 projects per request"}
	}

	for i := range inputs {
		inputs[i].Name = strings.TrimSpace(inputs[i].Name)
		if msg := validateProjectName(inputs[i].Name); msg != "" {
			return nil, &batchValidationError{msg}
		}
		if !validFrameworks[inputs[i].Framework] {
			traceway.CaptureMessage("Invalid framework received in batch create: " + inputs[i].Framework)
			return nil, &batchValidationError{invalidFrameworkMessage}
		}
	}

	existing, err := transactional.ProjectRepository.FindByOrganizationId(tx, orgId)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*models.Project, len(existing))
	for _, p := range existing {
		byName[p.Name] = p
	}

	results := make([]BatchProjectResult, 0, len(inputs))
	for _, input := range inputs {
		if project, ok := byName[input.Name]; ok {
			results = append(results, BatchProjectResult{Project: project, Status: "existing"})
			continue
		}

		if ProjectLimitHook != nil {
			if err := ProjectLimitHook(tx, orgId); err != nil {
				return nil, err
			}
		}

		project, err := transactional.ProjectRepository.CreateWithOrganization(tx, input.Name, input.Framework, orgId)
		if err != nil {
			return nil, err
		}
		if err := populateDefaultDashboards(tx, project, &createdBy); err != nil {
			return nil, err
		}
		byName[project.Name] = project
		results = append(results, BatchProjectResult{Project: project, Status: "created"})
	}
	return results, nil
}

type BatchProjectResponseItem struct {
	Id             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Framework      string    `json:"framework"`
	Token          string    `json:"token"`
	SourceMapToken *string   `json:"sourceMapToken,omitempty"`
	BackendUrl     string    `json:"backendUrl"`
	Status         string    `json:"status"`
}

func toBatchProjectResponse(results []BatchProjectResult) []BatchProjectResponseItem {
	items := make([]BatchProjectResponseItem, 0, len(results))
	for _, r := range results {
		items = append(items, BatchProjectResponseItem{
			Id:             r.Project.Id,
			Name:           r.Project.Name,
			Framework:      r.Project.Framework,
			Token:          r.Project.Token,
			SourceMapToken: r.Project.SourceMapToken,
			BackendUrl:     models.GetBackendUrl(),
			Status:         r.Status,
		})
	}
	return items
}

// cacheCreatedProjectsOnCommit registers the freshly created projects with the
// ingest token cache once the request transaction has committed.
func cacheCreatedProjectsOnCommit(ctx *gin.Context, results []BatchProjectResult) {
	created := []*models.Project{}
	for _, r := range results {
		if r.Status == "created" {
			created = append(created, r.Project)
		}
	}
	if len(created) == 0 {
		return
	}
	middleware.OnCommit(ctx, func() {
		for _, p := range created {
			cache.ProjectCache.AddProject(p)
		}
	})
}

func respondBatchCreateError(ctx *gin.Context, err error) {
	var validationErr *batchValidationError
	if errors.As(err, &validationErr) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": validationErr.Message})
		return
	}
	var limitErr *LimitExceededError
	if errors.As(err, &limitErr) {
		ctx.JSON(http.StatusUnprocessableEntity, gin.H{"error": limitErr.Message})
		return
	}
	ctx.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("batch create projects: %w", err))
}

type BatchCreateProjectsRequest struct {
	OrganizationId int                 `json:"organizationId" binding:"required"`
	Projects       []BatchProjectInput `json:"projects" binding:"required"`
}

// BatchCreateProjects is the JWT-authenticated batch path. Unlike POST
// /projects it needs no existing ?projectId=, so zero-project accounts (the
// registration wizard's manual step) can use it.
func (p projectController) BatchCreateProjects(ctx *gin.Context) {
	var request BatchCreateProjectsRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := db.GetTx(ctx)
	if !requireOrgWrite(ctx, tx, request.OrganizationId) {
		return
	}

	results, err := batchCreateProjects(tx, request.OrganizationId, middleware.GetUserId(ctx), request.Projects)
	if err != nil {
		respondBatchCreateError(ctx, err)
		return
	}

	cacheCreatedProjectsOnCommit(ctx, results)
	ctx.JSON(http.StatusCreated, gin.H{"projects": toBatchProjectResponse(results)})
}
