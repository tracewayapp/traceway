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

func (p projectController) ListProjects(c *gin.Context) {
	userId := middleware.GetUserId(c)

	projectsWithBackendUrl, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return repositories.ProjectRepository.FindByUserId(tx, userId)
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

	cache.ProjectCache.AddProject(project)

	c.JSON(http.StatusCreated, project.ToProjectWithBackendUrl())
}

var ProjectController = projectController{}
