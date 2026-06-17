package controllers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type distributedTraceController struct{}

type distributedTraceRequest struct {
	RecordedAt *time.Time `json:"recordedAt"`
}

type DistributedTraceNode struct {
	ProjectId   uuid.UUID              `json:"projectId"`
	ProjectName string                 `json:"projectName"`
	TraceType   string                 `json:"traceType"`
	Endpoint    *models.Endpoint       `json:"endpoint,omitempty"`
	Task        *models.Task           `json:"task,omitempty"`
	AiTrace     *models.AiTrace        `json:"aiTrace,omitempty"`
	Spans       []models.Span          `json:"spans"`
	Exception   *EndpointExceptionInfo `json:"exception,omitempty"`
}

type DistributedTraceResponse struct {
	DistributedTraceId string                 `json:"distributedTraceId"`
	Nodes              []DistributedTraceNode `json:"nodes"`
}

func (d distributedTraceController) GetDistributedTrace(c *gin.Context) {
	distributedTraceIdStr := c.Param("distributedTraceId")
	distributedTraceId, err := uuid.Parse(distributedTraceIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid distributedTraceId"})
		return
	}

	var request distributedTraceRequest
	_ = c.ShouldBindJSON(&request)

	userId := middleware.GetUserId(c)

	projects, err := db.ExecuteTransaction(func(tx *sql.Tx) ([]*models.Project, error) {
		return repositories.ProjectRepository.FindByUserId(tx, userId)
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to load user projects: %w", err))
		return
	}

	if len(projects) == 0 {
		c.JSON(http.StatusOK, DistributedTraceResponse{
			DistributedTraceId: distributedTraceIdStr,
			Nodes:              []DistributedTraceNode{},
		})
		return
	}

	projectIds := make([]uuid.UUID, len(projects))
	projectNameMap := make(map[uuid.UUID]string, len(projects))
	for i, p := range projects {
		projectIds[i] = p.Id
		projectNameMap[p.Id] = p.Name
	}

	ctx := context.Background()

	endpoints, err := repositories.EndpointRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query endpoints: %w", err))
		return
	}

	tasks, err := repositories.TaskRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query tasks: %w", err))
		return
	}

	aiTraces, err := repositories.AiTraceRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query ai traces: %w", err))
		return
	}

	exceptions, err := repositories.ExceptionStackTraceRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query exceptions: %w", err))
		return
	}

	exceptionByTraceId := make(map[uuid.UUID]*EndpointExceptionInfo)
	for _, exc := range exceptions {
		if exc.TraceId != nil {
			if _, exists := exceptionByTraceId[*exc.TraceId]; !exists {
				exceptionByTraceId[*exc.TraceId] = &EndpointExceptionInfo{
					ExceptionHash: exc.ExceptionHash,
					StackTrace:    exc.StackTrace,
					RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				}
			}
		}
	}

	matchedIds := make(map[uuid.UUID]bool)
	for _, ep := range endpoints {
		matchedIds[ep.Id] = true
	}
	for _, t := range tasks {
		matchedIds[t.Id] = true
	}
	for _, a := range aiTraces {
		matchedIds[a.Id] = true
	}

	var nodes []DistributedTraceNode

	for _, ep := range endpoints {
		node := DistributedTraceNode{
			ProjectId:   ep.ProjectId,
			ProjectName: projectNameMap[ep.ProjectId],
			TraceType:   "endpoint",
			Endpoint:    &ep,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[ep.Id],
		}
		nodes = append(nodes, node)
	}

	for _, t := range tasks {
		node := DistributedTraceNode{
			ProjectId:   t.ProjectId,
			ProjectName: projectNameMap[t.ProjectId],
			TraceType:   "task",
			Task:        &t,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[t.Id],
		}
		nodes = append(nodes, node)
	}

	for _, a := range aiTraces {
		node := DistributedTraceNode{
			ProjectId:   a.ProjectId,
			ProjectName: projectNameMap[a.ProjectId],
			TraceType:   "ai_trace",
			AiTrace:     &a,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[a.Id],
		}
		nodes = append(nodes, node)
	}

	for _, exc := range exceptions {
		if exc.TraceId != nil && matchedIds[*exc.TraceId] {
			continue
		}
		nodes = append(nodes, DistributedTraceNode{
			ProjectId:   exc.ProjectId,
			ProjectName: projectNameMap[exc.ProjectId],
			TraceType:   "exception",
			Spans:       []models.Span{},
			Exception: &EndpointExceptionInfo{
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			},
		})
	}

	if nodes == nil {
		nodes = []DistributedTraceNode{}
	}

	c.JSON(http.StatusOK, DistributedTraceResponse{
		DistributedTraceId: distributedTraceIdStr,
		Nodes:              nodes,
	})
}

var DistributedTraceController = distributedTraceController{}
