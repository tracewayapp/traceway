package controllers

import (
	"backend/app/cache"
	"backend/app/models"
	"backend/app/pgdb"
	"backend/app/repositories"
	"backend/app/services"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

type authController struct{}

func (a authController) Login(c *gin.Context) {
	var request models.LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.LoginResponse, error) {
		user, err := repositories.UserRepository.FindByEmail(tx, request.Email)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, nil
		}

		if !services.CheckPassword(request.Password, user.Password) {
			return nil, nil
		}

		token, err := services.GenerateToken(user.Id, user.Email)
		if err != nil {
			return nil, err
		}

		return &models.LoginResponse{
			Token: token,
			User:  user.ToResponse(),
		}, nil
	})

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if result == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (a authController) Register(c *gin.Context) {
	var request models.RegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.RegisterResponse, error) {
		// Check if email already exists
		exists, err := repositories.UserRepository.EmailExists(tx, request.Email)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, &EmailExistsError{}
		}

		// Hash password
		hashedPassword, err := services.HashPassword(request.Password)
		if err != nil {
			return nil, err
		}

		// Create user
		user, err := repositories.UserRepository.Create(tx, request.Email, request.Name, hashedPassword)
		if err != nil {
			return nil, err
		}

		// Create organization
		org, err := repositories.OrganizationRepository.Create(tx, request.OrganizationName)
		if err != nil {
			return nil, err
		}

		// Add user to organization as owner
		_, err = repositories.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner")
		if err != nil {
			return nil, err
		}

		// Create project with organization
		project, err := repositories.ProjectRepository.CreateWithOrganization(tx, request.ProjectName, request.Framework, org.Id)
		if err != nil {
			return nil, err
		}

		// Generate JWT token
		token, err := services.GenerateToken(user.Id, user.Email)
		if err != nil {
			return nil, err
		}

		return &models.RegisterResponse{
			Token:   token,
			User:    user.ToResponse(),
			Project: project.ToWithToken(),
		}, nil
	})

	if err != nil {
		if _, ok := err.(*EmailExistsError); ok {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Add project to cache
	cache.ProjectCache.AddProject(&models.Project{
		Id:             result.Project.Id,
		Name:           result.Project.Name,
		Token:          result.Project.Token,
		Framework:      result.Project.Framework,
		OrganizationId: result.Project.OrganizationId,
	})

	c.JSON(http.StatusCreated, result)
}

func (a authController) Me(c *gin.Context) {
	userId, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	result, err := pgdb.ExecuteTransaction(func(tx *sql.Tx) (*models.UserResponse, error) {
		user, err := repositories.UserRepository.FindById(tx, userId.(int))
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, nil
		}
		response := user.ToResponse()
		return &response, nil
	})

	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, result)
}

type EmailExistsError struct{}

func (e *EmailExistsError) Error() string {
	return "email already exists"
}

var AuthController = authController{}
