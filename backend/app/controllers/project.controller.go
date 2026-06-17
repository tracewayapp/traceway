package controllers

import (
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

// Valid framework values
var validFrameworks = map[string]bool{
	// Go frameworks
	"gin":      true,
	"fiber":    true,
	"chi":      true,
	"fasthttp": true,
	"stdlib":   true,
	"custom":   true,
	// JavaScript frameworks
	"react":   true,
	"svelte":  true,
	"vuejs":   true,
	"nextjs":  true,
	"nestjs":  true,
	"express": true,
	"remix":          true,
	"jquery":         true,
	"react-native":   true,
	"hono":           true,
	"cloudflare":     true,
	"opentelemetry":  true,
	// PHP frameworks
	"symfony":        true,
	"laravel":        true,
	// Python frameworks
	"django":         true,
	// Mobile frameworks
	"flutter":        true,
	"android":        true,
	"ios":            true,
}

// Project name validation regex: allows alphanumeric, spaces, hyphens, and underscores
var projectNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)

type projectController struct{}

type CreateProjectRequest struct {
	Name      string `json:"name" binding:"required"`
	Framework string `json:"framework" binding:"required"`
}

type UpdateProjectRequest struct {
	Name                    string    `json:"name" binding:"required"`
	Framework               string    `json:"framework" binding:"required"`
	DropHealthyHealthchecks *bool     `json:"dropHealthyHealthchecks"`
	HealthcheckPaths        *[]string `json:"healthcheckPaths"`
}

type DeleteProjectRequest struct {
	Name string `json:"name" binding:"required"`
}

func (p projectController) ListProjects(c *gin.Context) {
	userId := middleware.GetUserId(c)

	projectsWithBackendUrl, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.ProjectWithBackendUrl, error) {
		return repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, userId)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error fetching projects: %w", err))
		return
	}

	c.JSON(http.StatusOK, projectsWithBackendUrl)
}

// CreateProject creates a new project and returns it with its token
func (p projectController) CreateProject(c *gin.Context) {
	var request CreateProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate project name
	nameLen := utf8.RuneCountInString(request.Name)
	if nameLen < 1 || nameLen > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name must be between 1 and 100 characters"})
		return
	}
	if !projectNameRegex.MatchString(request.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Project name can only contain letters, numbers, spaces, hyphens, and underscores"})
		return
	}

	if !validFrameworks[request.Framework] {
		traceway.CaptureMessage("Invalid framework received: " + request.Framework)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, laravel, django, flutter, android, ios"})
		return
	}

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		currentProject, err := repositories.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return nil, err
		}
		if currentProject == nil || currentProject.OrganizationId == nil {
			return nil, fmt.Errorf("current project has no organization")
		}
		return repositories.ProjectRepository.CreateWithOrganization(tx, request.Name, request.Framework, *currentProject.OrganizationId)
	})
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error creating a project: %w", err))
		return
	}

	cache.ProjectCache.AddProject(project)

	c.JSON(http.StatusCreated, project.ToProjectWithBackendUrl())
}

func (p projectController) UpdateProject(c *gin.Context) {
	var request UpdateProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	nameLen := utf8.RuneCountInString(request.Name)
	if nameLen < 1 || nameLen > 100 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project name must be between 1 and 100 characters"})
		return
	}
	if !projectNameRegex.MatchString(request.Name) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project name can only contain letters, numbers, spaces, hyphens, and underscores"})
		return
	}
	if !validFrameworks[request.Framework] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, laravel, django, flutter, android, ios"})
		return
	}

	var healthcheckPaths *[]string
	if request.HealthcheckPaths != nil {
		if len(*request.HealthcheckPaths) > 50 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "At most 50 healthcheck paths are allowed"})
			return
		}
		cleaned := make([]string, 0, len(*request.HealthcheckPaths))
		for _, path := range *request.HealthcheckPaths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if utf8.RuneCountInString(path) > 200 {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Healthcheck paths must be at most 200 characters"})
				return
			}
			base := strings.TrimSuffix(strings.TrimPrefix(path, "*"), "*")
			if !strings.HasPrefix(base, "/") {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Healthcheck paths must start with / (a leading or trailing * is allowed as a wildcard)"})
				return
			}
			if strings.Contains(base, "*") {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Wildcards are only allowed as a single leading or trailing *"})
				return
			}
			if base == "/" && base != path {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Wildcard healthcheck paths need at least one character besides /"})
				return
			}
			cleaned = append(cleaned, path)
		}
		healthcheckPaths = &cleaned
	}

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	project, err := db.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return repositories.ProjectRepository.Update(tx, projectId, request.Name, request.Framework, request.DropHealthyHealthchecks, healthcheckPaths)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error updating project: %w", err))
		return
	}

	cache.ProjectCache.UpdateProject(project)

	c.JSON(http.StatusOK, project.ToProjectWithBackendUrl())
}

func (p projectController) DeleteProject(c *gin.Context) {
	var request DeleteProjectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	nameMatched, err := db.ExecuteTransaction(func(tx *sql.Tx) (bool, error) {
		project, err := repositories.ProjectRepository.FindById(tx, projectId)
		if err != nil {
			return false, err
		}
		if project == nil {
			return false, fmt.Errorf("project not found: %s", projectId)
		}
		if project.Name != request.Name {
			return false, nil
		}
		if err := repositories.ProjectRepository.Delete(tx, projectId); err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error deleting project: %w", err))
		return
	}
	if !nameMatched {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Project name does not match"})
		return
	}

	cache.ProjectCache.RemoveProject(projectId)

	c.JSON(http.StatusOK, gin.H{})
}

func (p projectController) GenerateSourceMapToken(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	token, err := db.ExecuteTransaction(func(tx *sql.Tx) (string, error) {
		return repositories.ProjectRepository.GenerateSourceMapToken(tx, projectId)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to generate source map token: %w", err))
		return
	}

	cache.ProjectCache.UpdateSourceMapToken(projectId, token)

	c.JSON(http.StatusOK, gin.H{"sourceMapToken": token})
}

var ProjectController = projectController{}
