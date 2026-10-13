package middleware

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

const (
	agentRouteForbiddenMessage = "This endpoint is not available to personal access tokens, the CLI, or MCP clients. Use the Traceway dashboard."
	agentWriteScopeMessage     = "This credential is read-only. Writes need a write token: create one in the dashboard, or log in with traceway-write."
	agentWriteDisabledMessage  = "Agent write access is turned off for this organization. An organization admin can turn it on in Settings > Agent Access."
)

func authorizeAgentRequest(c *gin.Context, identity *BearerIdentity) {
	access, ok := agentRouteAccess(c.Request.Method, c.FullPath())
	if !ok {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": agentRouteForbiddenMessage})
		return
	}
	if access == agentRead {
		c.Next()
		return
	}

	if identity.Scope != models.ScopeWrite {
		c.Header("WWW-Authenticate", fmt.Sprintf(`Bearer error="insufficient_scope", scope=%q`, models.ScopeWrite))
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": agentWriteScopeMessage})
		return
	}

	projectId := extractProjectId(c)
	if projectId == uuid.Nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "projectId is required"})
		return
	}

	type policy struct {
		organizationId int
		enabled        bool
		found          bool
	}
	p, err := db.ExecuteTransaction(func(tx *sql.Tx) (policy, error) {
		orgId, enabled, found, err := transactional.OrganizationRepository.AgentWritePolicyForProject(tx, projectId, identity.UserId)
		return policy{orgId, enabled, found}, err
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("resolve agent write policy: %w", err))
		return
	}
	if !p.found {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "You do not have access to this project"})
		return
	}
	if !p.enabled {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": agentWriteDisabledMessage})
		recordAgentWrite(c, identity, p.organizationId, projectId)
		return
	}

	c.Next()
	recordAgentWrite(c, identity, p.organizationId, projectId)
}

func recordAgentWrite(c *gin.Context, identity *BearerIdentity, organizationId int, projectId uuid.UUID) {
	event := &models.AgentWriteEvent{
		Id:             uuid.New().String(),
		OrganizationId: organizationId,
		ProjectId:      projectId.String(),
		UserId:         identity.UserId,
		CredentialKind: identity.CredentialKind,
		CredentialRef:  identity.CredentialRef,
		Method:         c.Request.Method,
		Route:          c.FullPath(),
		Path:           c.Request.URL.Path,
		StatusCode:     c.Writer.Status(),
		CreatedAt:      time.Now().UTC(),
	}
	_, err := db.ExecuteTransaction(func(tx *sql.Tx) (struct{}, error) {
		return struct{}{}, transactional.AgentWriteEventRepository.Insert(tx, event)
	})
	if err != nil {
		traceway.CaptureException(fmt.Errorf("record agent write event (route=%s): %w", event.Route, err))
	}
}
