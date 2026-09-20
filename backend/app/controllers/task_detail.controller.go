package controllers

import (
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type taskDetailController struct{}

type taskDetailRequest struct {
	RecordedAt *time.Time `json:"recordedAt"`
}

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
	Task            *models.Task            `json:"task"`
	SpanGraphStatus *models.SpanGraphStatus `json:"spanGraphStatus"`
	Spans           []models.Span           `json:"spans"`
	HasSpans        bool                    `json:"hasSpans"`
	Exception       *TaskExceptionInfo      `json:"exception,omitempty"`
	Messages        []TaskMessageInfo       `json:"messages"`
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

	var request taskDetailRequest
	_ = c.ShouldBindJSON(&request)

	// Get task
	span := traceway.StartSpan(c, "loading task")
	task, err := telemetry.TaskRepository.FindById(c, projectId, taskId, request.RecordedAt)
	if task == nil && err == nil && request.RecordedAt != nil {
		task, err = telemetry.TaskRepository.FindById(c, projectId, taskId, nil)
	}
	span.End()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading task: %w", err))
		return
	}
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	recordedAt := task.RecordedAt

	// Get spans (flat list ordered by start_time)
	span = traceway.StartSpan(c, "loading spans")
	graph, err := telemetry.SpanRepository.FindGraph(c, shared.NewSpanLookup(projectId, task.TraceId, task.SpanId, recordedAt))
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans: %w", err))
		return
	}

	// Get all linked exceptions and messages
	var exceptionInfo *TaskExceptionInfo
	var messages []TaskMessageInfo

	span = traceway.StartSpan(c, "loading exceptions")
	allExceptions, err := findOccurrenceExceptions(c, projectId, task.TraceId, task.SpanId, &recordedAt, graph.Spans)
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

	c.JSON(http.StatusOK, TaskDetailResponse{
		Task:            task,
		SpanGraphStatus: &graph.Status,
		Spans:           graph.Spans,
		HasSpans:        len(graph.Spans) > 0,
		Exception:       exceptionInfo,
		Messages:        messages,
	})
}

var TaskDetailController = taskDetailController{}
