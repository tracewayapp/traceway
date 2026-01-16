package controllers

import (
	"backend/app/cache"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"regexp"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// ListProjects returns all projects without tokens
func (p projectController) ListProjects(c *gin.Context) {
	projects := cache.ProjectCache.GetAll()

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

	project, err := repositories.ProjectRepository.Create(c, request.Name, request.Framework)
	if err != nil {
		panic(err)
	}

	// Add to cache
	cache.ProjectCache.AddProject(project)

	// Return with token since this is creation
	c.JSON(http.StatusCreated, project.ToWithToken())
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
