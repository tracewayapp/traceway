package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tracewayapp/traceway/backend/app/cache"
	"github.com/tracewayapp/traceway/backend/app/config"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"

	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	traceway "go.tracewayapp.com"
)

type oauthController struct{}

type oauthProvidersResponse struct {
	Providers            []string          `json:"providers"`
	ProviderLabels       map[string]string `json:"providerLabels"`
	PasswordLoginEnabled bool              `json:"passwordLoginEnabled"`
}

type finishOAuthSetupRequest struct {
	OrganizationName string `json:"organizationName" binding:"required"`
	Timezone         string `json:"timezone" binding:"required"`
	ProjectName      string `json:"projectName" binding:"required"`
	Framework        string `json:"framework" binding:"required"`
}

func (a oauthController) ListProviders(c *gin.Context) {
	passwordEnabled := config.Config.DisablePasswordLogin != "true"
	if services.OAuthService == nil {
		c.JSON(http.StatusOK, oauthProvidersResponse{Providers: []string{}, ProviderLabels: map[string]string{}, PasswordLoginEnabled: passwordEnabled})
		return
	}
	labels := map[string]string{}
	if name := services.OAuthService.OIDCDisplayName(); name != "" {
		labels["oidc"] = name
	}
	c.JSON(http.StatusOK, oauthProvidersResponse{
		Providers:            services.OAuthService.EnabledProviders(),
		ProviderLabels:       labels,
		PasswordLoginEnabled: passwordEnabled,
	})
}

func (a oauthController) Begin(c *gin.Context) {
	provider := c.Param("provider")
	if services.OAuthService == nil || !services.OAuthService.IsProviderEnabled(provider) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown OAuth provider"})
		return
	}

	req := c.Request.WithContext(context.WithValue(c.Request.Context(), gothic.ProviderParamKey, externalToGothProvider(provider)))
	gothic.BeginAuthHandler(c.Writer, req)
}

func (a oauthController) Callback(c *gin.Context) {
	provider := c.Param("provider")
	if services.OAuthService == nil || !services.OAuthService.IsProviderEnabled(provider) {
		a.redirectError(c, "oauth_failed")
		return
	}

	req := c.Request.WithContext(context.WithValue(c.Request.Context(), gothic.ProviderParamKey, externalToGothProvider(provider)))
	gothUser, err := gothic.CompleteUserAuth(c.Writer, req)
	if err != nil {
		traceway.CaptureException(fmt.Errorf("OAuth complete failed (provider=%s): %w", provider, err))
		a.redirectError(c, "oauth_failed")
		return
	}

	if gothUser.Email == "" {
		a.redirectError(c, "oauth_no_email")
		return
	}

	user, err := a.findOrCreateUser(c, provider, gothUser)
	if err != nil {
		return
	}

	tx := db.GetTx(c)

	var mappedRole string
	if provider == "oidc" && services.OAuthService.OIDCRoleClaimEnabled() {
		mappedRole = services.OAuthService.ResolveRole(gothUser.RawData)
	}

	memberships, err := repositories.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: load memberships: %w", err))
		return
	}
	needsSetup := len(memberships) == 0

	if needsSetup && config.Config.CloudMode != "true" && provider == "oidc" && services.OAuthService.OIDCAutoCreateEnabled() {
		org, err := a.resolveOIDCOrg(tx, gothUser.RawData)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: resolve org for auto-join: %w", err))
			return
		}
		if org != nil {
			role := "user"
			if mappedRole != "" {
				role = mappedRole
			}
			if _, err := repositories.OrganizationRepository.AddUser(tx, org.Id, user.Id, role); err != nil {
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: auto-join org: %w", err))
				return
			}
			needsSetup = false
		}
	} else if mappedRole != "" && !needsSetup {
		for _, m := range memberships {
			if m.Role == "owner" {
				continue
			}
			if err := repositories.OrganizationRepository.UpdateUserRole(tx, m.Id, user.Id, mappedRole); err != nil {
				c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: sync mapped role: %w", err))
				return
			}
		}
	}

	jwt, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: generate JWT: %w", err))
		return
	}

	target := fmt.Sprintf("%s/auth/callback#token=%s&needsSetup=%t",
		strings.TrimRight(config.Config.AppBaseURL, "/"),
		url.QueryEscape(jwt),
		needsSetup,
	)
	c.Redirect(http.StatusSeeOther, target)
}

func (a oauthController) findOrCreateUser(c *gin.Context, provider string, gothUser goth.User) (*models.User, error) {
	tracewayTx := db.GetTx(c)

	user, err := repositories.UserRepository.FindByOAuth(tracewayTx, provider, gothUser.UserID)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: lookup by provider: %w", err))
		return nil, err
	}
	if user != nil {
		return user, nil
	}

	existing, err := repositories.UserRepository.FindByEmailIgnoreCase(tracewayTx, gothUser.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: lookup by email: %w", err))
		return nil, err
	}

	if existing != nil {
		if err := repositories.UserRepository.LinkOAuth(tracewayTx, existing.Id, provider, gothUser.UserID, gothUser.AvatarURL); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: link provider: %w", err))
			return nil, err
		}
		return existing, nil
	}

	oidcAutoCreate := provider == "oidc" && services.OAuthService.OIDCAutoCreateEnabled()
	if config.Config.CloudMode != "true" && !oidcAutoCreate {
		hasOrg, err := repositories.OrganizationRepository.HasOrganizations(tracewayTx)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: count orgs: %w", err))
			return nil, err
		}
		if hasOrg {
			a.redirectError(c, "invite_required")
			return nil, errors.New("invite required")
		}
	}

	name := gothUser.Name
	if name == "" {
		name = gothUser.NickName
	}
	if name == "" {
		name = gothUser.Email
	}

	created, err := repositories.UserRepository.CreateOAuth(tracewayTx, gothUser.Email, name, provider, gothUser.UserID, gothUser.AvatarURL)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("OAuth callback: create user: %w", err))
		return nil, err
	}

	return created, nil
}

func (a oauthController) FinishSetup(c *gin.Context) {
	var request finishOAuthSetupRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if !validFrameworks[request.Framework] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Framework must be one of: gin, fiber, chi, fasthttp, stdlib, custom, react, svelte, vuejs, jquery, react-native, hono, cloudflare, opentelemetry, symfony, flutter, android, ios"})
		return
	}

	userId := middleware.GetUserId(c)
	if userId == 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	tx := db.GetTx(c)

	user, err := repositories.UserRepository.FindById(tx, userId)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: load user: %w", err))
		return
	}
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	memberships, err := repositories.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: load memberships: %w", err))
		return
	}
	if len(memberships) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Setup already complete"})
		return
	}

	if config.Config.CloudMode != "true" {
		hasOrg, err := repositories.OrganizationRepository.HasOrganizations(tx)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: count orgs: %w", err))
			return
		}
		if hasOrg {
			c.JSON(http.StatusConflict, gin.H{"error": "An organization already exists. Please ask an admin to invite you."})
			return
		}
	}

	org, err := repositories.OrganizationRepository.Create(tx, request.OrganizationName, request.Timezone)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: create org: %w", err))
		return
	}

	if _, err := repositories.OrganizationRepository.AddUser(tx, org.Id, user.Id, "owner"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: add user to org: %w", err))
		return
	}

	for _, hook := range PostRegistrationHooks {
		if err := hook(tx, org, user); err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: post-registration hook: %w", err))
			return
		}
	}

	project, err := repositories.ProjectRepository.CreateWithOrganization(tx, request.ProjectName, request.Framework, org.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: create project: %w", err))
		return
	}

	cache.ProjectCache.AddProject(&models.Project{
		Id:             project.Id,
		Name:           project.Name,
		Token:          project.Token,
		Framework:      project.Framework,
		OrganizationId: project.OrganizationId,
	})

	token, err := services.GenerateToken(user.Id, user.Email)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: regenerate token: %w", err))
		return
	}

	projects, err := repositories.ProjectRepository.FindAllWithBackendUrlByUserId(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: load projects: %w", err))
		return
	}

	organizations, err := repositories.OrganizationRepository.FindByUserIdWithRoles(tx, user.Id)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("FinishSetup: load orgs: %w", err))
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

func (a oauthController) redirectError(c *gin.Context, code string) {
	target := fmt.Sprintf("%s/login?error=%s", strings.TrimRight(config.Config.AppBaseURL, "/"), url.QueryEscape(code))
	c.Redirect(http.StatusSeeOther, target)
}

func (a oauthController) resolveOIDCOrg(tx *sql.Tx, rawData map[string]interface{}) (*models.Organization, error) {
	if claim := services.OAuthService.OIDCOrgClaim(); claim != "" {
		if val, ok := rawData[claim]; ok {
			if orgName, ok := val.(string); ok && orgName != "" {
				org, err := repositories.OrganizationRepository.FindByName(tx, orgName)
				if err != nil {
					return nil, err
				}
				if org != nil {
					return org, nil
				}
			}
		}
	}
	return repositories.OrganizationRepository.FindFirst(tx)
}

func externalToGothProvider(provider string) string {
	if provider == "oidc" {
		return "openid-connect"
	}
	return provider
}

var OAuthController = oauthController{}
