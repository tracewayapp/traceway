package controllers

import (
	"backend/app/cache"
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"database/sql"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

// Valid framework values
var validFrameworks = map[string]bool{
	"stdlib":   true,
	"fasthttp": true,
	"gin":      true,
	"fiber":    true,
	"chi":      true,
	"custom":   true,
}

// Project name validation regex: allows alphanumeric, spaces, hyphens, and underscores
var projectNameRegex = regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)

type projectController struct{}

type CreateProjectRequest struct {
	Name      string `json:"name" binding:"required"`
	Framework string `json:"framework" binding:"required"`
}

// ListProjects returns projects accessible by the authenticated user
func (p projectController) ListProjects(c *gin.Context) {
	userId := middleware.GetUserId(c)

	projects, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return repositories.ProjectRepository.FindByUserId(tx, userId)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error fetching projects: %w", err))
		return
	}

	// Convert to response format (without tokens)
	response := make([]models.ProjectResponse, len(projects))
	for i, proj := range projects {
		response[i] = proj.ToResponse()
	}

	c.JSON(http.StatusOK, response)
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

	// Validate framework
	if !validFrameworks[request.Framework] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: stdlib, fasthttp, gin, fiber, chi, custom"})
		return
	}

	project, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.Project, error) {
		return repositories.ProjectRepository.Create(tx, request.Name, request.Framework)
	})
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error creating a project: %w", err))
		return
	}

	// Add to cache
	cache.ProjectCache.AddProject(project)

	// Return with token since this is creation
	c.JSON(http.StatusCreated, (*project).ToWithToken())
}

// GetProject returns a project by ID with its token (for connection page)
func (p projectController) GetProject(c *gin.Context) {
	projectId, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	project := cache.ProjectCache.GetById(projectId)
	if project == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
		return
	}

	// Return with token for connection page
	c.JSON(http.StatusOK, project.ToWithToken())
}

var ProjectController = projectController{}
