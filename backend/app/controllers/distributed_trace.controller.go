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

	ctx := context.Background()

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
	var spanRefs []models.TraceRef

	for i := range endpoints {
		ep := &endpoints[i]
		nodes = append(nodes, DistributedTraceNode{
			ProjectId:   ep.ProjectId,
			ProjectName: projectNameMap[ep.ProjectId],
			TraceType:   "endpoint",
			Endpoint:    ep,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[ep.Id],
		})
		spanRefs = append(spanRefs, models.TraceRef{ProjectId: ep.ProjectId, TraceId: ep.Id, RecordedAt: ep.RecordedAt})
	}

	for i := range tasks {
		t := &tasks[i]
		nodes = append(nodes, DistributedTraceNode{
			ProjectId:   t.ProjectId,
			ProjectName: projectNameMap[t.ProjectId],
			TraceType:   "task",
			Task:        t,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[t.Id],
		})
		spanRefs = append(spanRefs, models.TraceRef{ProjectId: t.ProjectId, TraceId: t.Id, RecordedAt: t.RecordedAt})
	}

	for i := range aiTraces {
		a := &aiTraces[i]
		nodes = append(nodes, DistributedTraceNode{
			ProjectId:   a.ProjectId,
			ProjectName: projectNameMap[a.ProjectId],
			TraceType:   "ai_trace",
			AiTrace:     a,
			Spans:       []models.Span{},
			Exception:   exceptionByTraceId[a.Id],
		})
		spanRefs = append(spanRefs, models.TraceRef{ProjectId: a.ProjectId, TraceId: a.Id, RecordedAt: a.RecordedAt})
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

	// Each entity node owns the spans stored under its id as trace_id, the
	// same ownership the detail pages read, loaded here in one bounded query.
	spans, err := telemetry.SpanRepository.FindByTraceIds(ctx, spanRefs)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("failed to query spans: %w", err))
		return
	}
	attachSpans(nodes, spans)

	if nodes == nil {
		nodes = []DistributedTraceNode{}
	}

	c.JSON(http.StatusOK, DistributedTraceResponse{
		DistributedTraceId: distributedTraceIdStr,
		Nodes:              nodes,
	})
}

// attachSpans hands every span to the node whose entity id and project it was
// stored under. Exception-only nodes have no entity and keep their empty list.
func attachSpans(nodes []DistributedTraceNode, spans []models.Span) {
	byOwner := make(map[models.TraceRef][]models.Span)
	for _, s := range spans {
		key := models.TraceRef{ProjectId: s.ProjectId, TraceId: s.TraceId}
		byOwner[key] = append(byOwner[key], s)
	}
	for i := range nodes {
		owner, ok := nodes[i].spanOwner()
		if !ok {
			continue
		}
		if owned := byOwner[owner]; len(owned) > 0 {
			nodes[i].Spans = owned
		}
	}
}

func (n DistributedTraceNode) spanOwner() (models.TraceRef, bool) {
	switch {
	case n.Endpoint != nil:
		return models.TraceRef{ProjectId: n.ProjectId, TraceId: n.Endpoint.Id}, true
	case n.Task != nil:
		return models.TraceRef{ProjectId: n.ProjectId, TraceId: n.Task.Id}, true
	case n.AiTrace != nil:
		return models.TraceRef{ProjectId: n.ProjectId, TraceId: n.AiTrace.Id}, true
	}
	return models.TraceRef{}, false
}

var DistributedTraceController = distributedTraceController{}
