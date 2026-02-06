package controllers

import (
	"backend/app/cache"
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"backend/app/services"
	"database/sql"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

var PostRegistrationHooks []func(tx *sql.Tx, org *models.Organization, user *models.User) error

type authController struct{}

func (a authController) HasOrganizations(c *gin.Context) {
	tx := middleware.GetTx(c)
	hasOrganizations, err := repositories.OrganizationRepository.HasOrganizations(tx)

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"hasOrganizations": hasOrganizations})
}

func (a authController) Login(c *gin.Context) {
	var request models.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tx := middleware.GetTx(c)

	user, err := repositories.UserRepository.FindByEmail(tx, request.Email)
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

	projects, err := repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	organizations, err := repositories.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
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

	tx := middleware.GetTx(c)

	// if we're not in the cloud mode only a single organization is allowed
	if os.Getenv("CLOUD_MODE") != "true" {
		hasOrganizations, err := repositories.OrganizationRepository.HasOrganizations(tx)

		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		if hasOrganizations {
			c.JSON(http.StatusConflict, gin.H{"error": "You already have an Organization registered. Please use login."})
			return
		}
	}

	exists, err := repositories.UserRepository.EmailExists(tx, request.Email)
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

	user, err := repositories.UserRepository.Create(tx, request.Email, request.Name, hashedPassword)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	org, err := repositories.OrganizationRepository.Create(tx, request.OrganizationName, request.Timezone)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	_, err = repositories.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner")
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

	if !validFrameworks[request.Framework] {
		traceway.CaptureMessage("Invalid framework received during registration: " + request.Framework)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, nextjs, nestjs, express, remix"})
		return
	}

	project, err := repositories.ProjectRepository.CreateWithOrganization(tx, request.ProjectName, request.Framework, org.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	projects, err := repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// cache can get out of sync here
	// the transaction for creating the project is not guaranteed to have
	// finished when this is inserted into cache
	cache.ProjectCache.AddProject(&models.Project{
		Id:             project.Id,
		Name:           project.Name,
		Token:          project.Token,
		Framework:      project.Framework,
		OrganizationId: project.OrganizationId,
	})

	organizations, err := repositories.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusCreated, &models.RegisterResponse{
		Token:         token,
		User:          user.ToResponse(),
		Project:       *project.ToProjectWithBackendUrl(),
		Projects:      projects,
		Organizations: organizations,
	})
}

var AuthController = authController{}
