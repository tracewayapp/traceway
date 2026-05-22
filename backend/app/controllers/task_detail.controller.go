package controllers

import (
	"net/http"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type taskDetailController struct{}

type TaskExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type TaskMessageInfo struct {
	Id            uuid.UUID         `json:"id"`
	ExceptionHash string            `json:"exceptionHash"`
	StackTrace    string            `json:"stackTrace"`
	RecordedAt    string            `json:"recordedAt"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

type TaskDetailResponse struct {
	Task          *models.Task          `json:"task"`
	Spans         []EnrichedSpan        `json:"spans"`
	HasSpans      bool                  `json:"hasSpans"`
	Exception     *TaskExceptionInfo    `json:"exception,omitempty"`
	Messages      []TaskMessageInfo     `json:"messages"`
	ParentRef     *ParentRefResponse    `json:"parentRef,omitempty"`
	ChildEntities []ChildEntityResponse `json:"childEntities"`
}

func (t taskDetailController) GetTaskDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	taskId, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid taskId"})
		return
	}

	// Get task
	span := traceway.StartSpan(c, "loading task")
	task, err := repositories.TaskRepository.FindById(c, projectId, taskId)
	span.End()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading task: %w", err))
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Load only spans whose entity_id resolves to this task, so a producer span
	// (or the originating request's internal spans) in the same trace doesn't
	// leak into this waterfall. Historical rows (pre-Phase-2) have entity_id
	// NULL — recognise them by the Phase-1 backfill invariant id == trace_id and
	// fall back to the trace-wide query so old detail pages keep working until
	// the operator runs the CH UPDATE to populate entity_id.
	span = traceway.StartSpan(c, "loading spans")
	spans, err := repositories.SpanRepository.FindByEntityId(c, projectId, task.Id)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans: %w", err))
		return
	}
	if len(spans) == 0 && task.Id == task.TraceId {
		span = traceway.StartSpan(c, "loading spans (legacy fallback)")
		spans, err = repositories.SpanRepository.FindByTraceId(c, projectId, task.TraceId)
		span.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans (legacy fallback): %w", err))
			return
		}
	}

	spanIds := make([]uuid.UUID, 0, len(spans))
	for _, s := range spans {
		spanIds = append(spanIds, s.Id)
	}
	aiRefs, err := repositories.AiTraceRepository.FindBySpanIds(c, projectId, spanIds)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading linked ai traces: %w", err))
		return
	}
	enrichedSpans := make([]EnrichedSpan, 0, len(spans))
	for _, s := range spans {
		es := EnrichedSpan{Span: s}
		if ref, ok := aiRefs[s.Id]; ok {
			id := ref.Id
			name := ref.TraceName
			es.LinkedAiTraceId = &id
			es.LinkedAiTraceName = &name
		}
		enrichedSpans = append(enrichedSpans, es)
	}

	// Get all linked exceptions and messages
	var exceptionInfo *TaskExceptionInfo
	var messages []TaskMessageInfo

	span = traceway.StartSpan(c, "loading exceptions")
	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, task.TraceId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading allExceptions: %w", err))
		return
	}

	for _, exc := range allExceptions {
		if exc.IsMessage {
			// Add to messages list
			messages = append(messages, TaskMessageInfo{
				Id:            exc.Id,
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				Attributes:    exc.Attributes,
			})
		} else if exceptionInfo == nil {
			// Only take the first actual exception
			exceptionInfo = &TaskExceptionInfo{
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			}
		}
	}

	if messages == nil {
		messages = []TaskMessageInfo{}
	}

	var parentRefResp *ParentRefResponse
	if task.ParentSpanId != nil {
		ref, err := repositories.FindParentRef(c, projectId, *task.ParentSpanId)
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading parent ref: %w", err))
			return
		}
		if ref != nil {
			parentRefResp = &ParentRefResponse{Kind: ref.Kind, Id: ref.Id, Name: ref.Name, TraceId: ref.TraceId}
		}
	}

	// See endpoint_detail.controller.go for the rationale on subtreeIds + child
	// entities — same shape applies here so a task that dispatches another task,
	// or calls an LLM, surfaces those entities in this task's waterfall.
	subtreeIds := make([]uuid.UUID, 0, len(spans)+1)
	subtreeIds = append(subtreeIds, task.Id)
	for _, s := range spans {
		subtreeIds = append(subtreeIds, s.Id)
	}
	span = traceway.StartSpan(c, "loading child entities")
	children, err := repositories.FindChildEntitiesBySpanIds(c, projectId, subtreeIds)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading child entities: %w", err))
		return
	}

	c.JSON(http.StatusOK, TaskDetailResponse{
		Task:          task,
		Spans:         enrichedSpans,
		HasSpans:      len(enrichedSpans) > 0,
		Exception:     exceptionInfo,
		Messages:      messages,
		ParentRef:     parentRefResp,
		ChildEntities: toChildEntityResponses(children),
	})
}

var TaskDetailController = taskDetailController{}
