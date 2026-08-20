package controllers

import (
	"database/sql"
	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
	"github.com/tracewayapp/traceway/backend/app/services"
	"net/http"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

var PostRegistrationHooks []func(tx *sql.Tx, org *models.Organization, user *models.User) error

type authController struct{}

func (a authController) HasOrganizations(c *gin.Context) {
	tx := db.GetTx(c)
	hasOrganizations, err := transactional.OrganizationRepository.HasOrganizations(tx)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"hasOrganizations": hasOrganizations})
}

func (a authController) Login(c *gin.Context) {
	if config.Config.DisablePasswordLogin == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Password login is disabled. Please use SSO to sign in."})
		return
	}

	var request models.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	tx := db.GetTx(c)

	user, err := transactional.UserRepository.FindByEmail(tx, request.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	if !services.CheckPassword(request.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	projects, err := transactional.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	organizations, err := transactional.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, &models.LoginResponse{
		Token:         token,
		User:          user.ToResponse(),
		Projects:      projects,
		Organizations: organizations,
	})
}

func (a authController) Register(c *gin.Context) {
	if config.Config.DisablePasswordLogin == "true" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Password login is disabled. Please use SSO to sign in."})
		return
	}

	var request models.RegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if services.TurnstileService.IsEnabled() {
		if err := services.TurnstileService.Verify(request.CaptchaToken, c.ClientIP()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	tx := db.GetTx(c)

	// if we're not in the cloud mode only a single organization is allowed
	if config.Config.CloudMode != "true" {
		hasOrganizations, err := transactional.OrganizationRepository.HasOrganizations(tx)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if hasOrganizations {
			c.JSON(http.StatusConflict, gin.H{"error": "You already have an Organization registered. Please use login."})
			return
		}
	}

	exists, err := transactional.UserRepository.EmailExists(tx, request.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		return
	}

	hashedPassword, err := services.HashPassword(request.Password)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	user, err := transactional.UserRepository.Create(tx, request.Email, request.Name, hashedPassword)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	org, err := transactional.OrganizationRepository.Create(tx, request.OrganizationName, request.Timezone)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = transactional.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	for _, hook := range PostRegistrationHooks {
		if err := hook(tx, org, user); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	wantsProject := request.ProjectName != "" || request.Framework != ""
	if wantsProject && (request.ProjectName == "" || request.Framework == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "projectName and framework must be provided together"})
		return
	}

	var project *models.Project
	if wantsProject {
		if !validFrameworks[request.Framework] {
			traceway.CaptureMessage("Invalid framework received during registration: " + request.Framework)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, flutter, android, ios"})
			return
		}

		project, err = transactional.ProjectRepository.CreateWithOrganization(tx, request.ProjectName, request.Framework, org.Id)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	projects, err := transactional.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	var projectWithUrl *models.ProjectWithBackendUrl
	if project != nil {
		cache.ProjectCache.AddProject(&models.Project{
			Id:             project.Id,
			Name:           project.Name,
			Token:          project.Token,
			Framework:      project.Framework,
			OrganizationId: project.OrganizationId,
		})
		projectWithUrl = project.ToProjectWithBackendUrl()
	}

	organizations, err := transactional.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, &models.RegisterResponse{
		Token:         token,
		User:          user.ToResponse(),
		Project:       projectWithUrl,
		Projects:      projects,
		Organizations: organizations,
	})
}

func (a authController) LoginBundle(c *gin.Context) {
	tx := db.GetTx(c)

	userId := middleware.GetUserId(c)
	if userId == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	user, err := transactional.UserRepository.FindById(tx, userId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("LoginBundle: load user: %w", err))
		return
	}
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	projects, err := transactional.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("LoginBundle: load projects: %w", err))
		return
	}

	organizations, err := transactional.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("LoginBundle: load orgs: %w", err))
		return
	}

	c.JSON(http.StatusOK, &models.LoginResponse{
		User:          user.ToResponse(),
		Projects:      projects,
		Organizations: organizations,
	})
}

var AuthController = authController{}
