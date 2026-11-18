package controllers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/db"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/transactional"

	"github.com/tracewayapp/traceway/backend/v2/app/middleware"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"

	"github.com/gin-gonic/gin"
)

type sessionController struct{}

type SessionAttributeFilter struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Exclude  bool   `json:"exclude"`
	Contains bool   `json:"contains"`
}

type SessionSearchRequest struct {
	TriggerId string `json:"triggerId"`
	// RuleId narrows the list to sessions recorded by a trigger of that Data Collection rule.
	RuleId           int                      `json:"ruleId"`
	FromDate         time.Time                `json:"fromDate"`
	ToDate           time.Time                `json:"toDate"`
	OrderBy          string                   `json:"orderBy"`
	SortDirection    string                   `json:"sortDirection"`
	Search           string                   `json:"search"`
	AttributeFilters []SessionAttributeFilter `json:"attributeFilters"`
	Pagination       PaginationParams         `json:"pagination"`
}

func (s sessionController) FindAllSessions(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	var request SessionSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		middleware.RejectBindError(c, err, err.Error())
		return
	}

	filters := make([]telemetry.SessionAttributeFilter, 0, len(request.AttributeFilters))
	for _, f := range request.AttributeFilters {
		if f.Key == "" {
			continue
		}
		filters = append(filters, telemetry.SessionAttributeFilter{Key: f.Key, Value: f.Value, Exclude: f.Exclude, Contains: f.Contains})
	}

	if len(request.TriggerId) > 64 {
		c.JSON(422, gin.H{"error": "Invalid trigger ID"})
		return
	}
	if request.TriggerId != "" {
		filters = append(filters, telemetry.SessionAttributeFilter{Key: "__recording_trigger", Value: request.TriggerId})
	}
	if request.RuleId != 0 {
		triggerIds, found, err := ruleTriggerIds(c, projectId, request.RuleId)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading the rule to filter sessions by: %w", err))
			return
		}
		if !found {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "The rule to filter by no longer exists"})
			return
		}
		// A rule without triggers recorded no session.
		if len(triggerIds) == 0 {
			c.JSON(http.StatusOK, PaginatedResponse[models.Session]{
				Data:       []models.Session{},
				Pagination: Pagination{Page: request.Pagination.Page, PageSize: request.Pagination.PageSize},
			})
			return
		}
		filters = append(filters, telemetry.SessionAttributeFilter{Key: "__recording_trigger", Value: strings.Join(triggerIds, ",")})
	}
	spanCtx, span := traceway.StartSpan(c, "loading sessions")
	sessions, total, err := telemetry.SessionRepository.FindAll(spanCtx, projectId, request.FromDate, request.ToDate, request.Pagination.Page, request.Pagination.PageSize, request.OrderBy, request.SortDirection, request.Search, filters)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading sessions: %w", err))
		return
	}

	if err := telemetry.LoadSessionRecordingReasons(c, projectId, sessions); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading session recording reasons: %w", err))
		return
	}
	c.JSON(http.StatusOK, PaginatedResponse[models.Session]{
		Data: sessions,
		Pagination: Pagination{
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			Total:      total,
			TotalPages: (total + int64(request.Pagination.PageSize) - 1) / int64(request.Pagination.PageSize),
		},
	})
}

// ruleTriggerIds returns the ids of the triggers a Data Collection rule of the project defines. A session is
// tied to a rule through the trigger that recorded it: the SDK reports the trigger, not the rule it came from.
func ruleTriggerIds(c *gin.Context, projectId uuid.UUID, ruleId int) ([]string, bool, error) {
	rule, err := db.ExecuteTransactionContext(c, func(tx *sql.Tx) (*models.SdkConfigRule, error) {
		return transactional.SdkConfigRepository.FindRule(tx, projectId, ruleId)
	})
	if err != nil || rule == nil {
		return nil, false, err
	}
	var settings struct {
		Triggers []struct {
			Id string `json:"id"`
		} `json:"triggers"`
	}
	if err := json.Unmarshal([]byte(rule.Settings), &settings); err != nil {
		return nil, true, nil
	}
	ids := make([]string, 0, len(settings.Triggers))
	for _, trigger := range settings.Triggers {
		if trigger.Id != "" && !strings.Contains(trigger.Id, ",") {
			ids = append(ids, trigger.Id)
		}
	}
	return ids, true, nil
}

var SessionController = sessionController{}

func (s sessionController) RecordingReasons(c *gin.Context) {
	project, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	reasons, err := telemetry.ListSessionRecordingReasons(c, project)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error listing session recording reasons: %w", err))
		return
	}
	c.JSON(200, gin.H{"reasons": reasons})
}
