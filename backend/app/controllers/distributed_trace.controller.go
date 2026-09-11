package controllers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"

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
		return transactional.ProjectRepository.FindByUserId(tx, userId)
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

	ctx := c.Request.Context()

	endpoints, err := telemetry.EndpointRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query endpoints: %w", err))
		return
	}

	tasks, err := telemetry.TaskRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query tasks: %w", err))
		return
	}

	aiTraces, err := telemetry.AiTraceRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query ai traces: %w", err))
		return
	}

	exceptions, err := telemetry.ExceptionStackTraceRepository.FindByDistributedTraceId(ctx, distributedTraceId, projectIds, request.RecordedAt)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query exceptions: %w", err))
		return
	}

	exceptionByTraceId := make(map[distributedTraceOwner]*EndpointExceptionInfo)
	for _, exc := range exceptions {
		if exc.TraceId != nil {
			if _, exists := exceptionByTraceId[distributedTraceOwner{exc.ProjectId, *exc.TraceId}]; !exists {
				exceptionByTraceId[distributedTraceOwner{exc.ProjectId, *exc.TraceId}] = &EndpointExceptionInfo{
					ExceptionHash: exc.ExceptionHash,
					StackTrace:    exc.StackTrace,
					RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				}
			}
		}
	}

	matchedIds := make(map[distributedTraceOwner]bool)
	for _, ep := range endpoints {
		matchedIds[distributedTraceOwner{ep.ProjectId, ep.Id}] = true
	}
	for _, t := range tasks {
		matchedIds[distributedTraceOwner{t.ProjectId, t.Id}] = true
	}
	for _, a := range aiTraces {
		matchedIds[distributedTraceOwner{a.ProjectId, a.Id}] = true
	}

	var nodes []DistributedTraceNode
	var lookups []telemetry.SpanLookup

	for _, ep := range endpoints {
		node := DistributedTraceNode{
			ProjectId:   ep.ProjectId,
			ProjectName: projectNameMap[ep.ProjectId],
			TraceType:   "endpoint",
			Endpoint:    &ep,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[distributedTraceOwner{ep.ProjectId, ep.Id}],
		}
		nodes = append(nodes, node)
		lookups = append(lookups, telemetry.SpanLookup{ProjectId: ep.ProjectId, TraceId: ep.Id, RecordedAt: &ep.RecordedAt})
	}

	for _, t := range tasks {
		node := DistributedTraceNode{
			ProjectId:   t.ProjectId,
			ProjectName: projectNameMap[t.ProjectId],
			TraceType:   "task",
			Task:        &t,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[distributedTraceOwner{t.ProjectId, t.Id}],
		}
		nodes = append(nodes, node)
		lookups = append(lookups, telemetry.SpanLookup{ProjectId: t.ProjectId, TraceId: t.Id, RecordedAt: &t.RecordedAt})
	}

	for _, a := range aiTraces {
		node := DistributedTraceNode{
			ProjectId:   a.ProjectId,
			ProjectName: projectNameMap[a.ProjectId],
			TraceType:   "ai_trace",
			AiTrace:     &a,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[distributedTraceOwner{a.ProjectId, a.Id}],
		}
		nodes = append(nodes, node)
		lookups = append(lookups, telemetry.SpanLookup{ProjectId: a.ProjectId, TraceId: a.Id, RecordedAt: &a.RecordedAt})
	}

	for _, exc := range exceptions {
		if exc.TraceId != nil && matchedIds[distributedTraceOwner{exc.ProjectId, *exc.TraceId}] {
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
		lookup := telemetry.SpanLookup{ProjectId: exc.ProjectId, RecordedAt: &exc.RecordedAt}
		if exc.TraceId != nil {
			lookup.TraceId = *exc.TraceId
		}
		lookups = append(lookups, lookup)
	}

	if err := loadDistributedTraceSpans(ctx, nodes, lookups); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query distributed trace spans: %w", err))
		return
	}

	if nodes == nil {
		nodes = []DistributedTraceNode{}
	}

	c.JSON(http.StatusOK, DistributedTraceResponse{
		DistributedTraceId: distributedTraceIdStr,
		Nodes:              nodes,
	})
}

type distributedTraceOwner struct {
	projectId uuid.UUID
	traceId   uuid.UUID
}

func loadDistributedTraceSpans(ctx context.Context, nodes []DistributedTraceNode, lookups []telemetry.SpanLookup) error {
	nodeIndexes := make(map[distributedTraceOwner][]int)
	queries := make([]telemetry.SpanLookup, 0, len(lookups))
	for i, lookup := range lookups {
		if lookup.TraceId == uuid.Nil {
			continue
		}
		key := distributedTraceOwner{lookup.ProjectId, lookup.TraceId}
		nodeIndexes[key] = append(nodeIndexes[key], i)
		queries = append(queries, lookup)
	}
	spans, err := telemetry.SpanRepository.FindByTraces(ctx, queries)
	if err != nil {
		return err
	}
	for _, span := range spans {
		for _, i := range nodeIndexes[distributedTraceOwner{span.ProjectId, span.TraceId}] {
			// A bulk result can cover multiple occurrences of the same ID. Keep
			// each node's window consistent with its individual detail endpoint.
			if lookups[i].RecordedAt != nil {
				from, to := shared.TraceWindowBounds(*lookups[i].RecordedAt)
				if span.RecordedAt.Before(from) || span.RecordedAt.After(to) {
					continue
				}
			}
			nodes[i].Spans = append(nodes[i].Spans, span)
		}
	}
	return nil
}

var DistributedTraceController = distributedTraceController{}
