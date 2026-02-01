package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

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
	Task     *models.Task       `json:"task"`
	Spans    []models.Span      `json:"spans"`
	HasSpans bool               `json:"hasSpans"`
	Exception   *TaskExceptionInfo `json:"exception,omitempty"`
	Messages    []TaskMessageInfo  `json:"messages"`
}

func (t taskDetailController) GetTaskDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	taskId, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid taskId"})
		return
	}

	// Get task
	span := traceway.StartSpan(c, "loading task")
	task, err := repositories.TaskRepository.FindById(c, projectId, taskId)
	span.End()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Get spans (flat list ordered by start_time)
	span = traceway.StartSpan(c, "loading spans")
	spans, err := repositories.SpanRepository.FindByTraceId(c, projectId, taskId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading spans: %w", err))
		return
	}

	// Get all linked exceptions and messages
	var exceptionInfo *TaskExceptionInfo
	var messages []TaskMessageInfo

	span = traceway.StartSpan(c, "loading exceptions")
	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTraceId(c, projectId, taskId)
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
		Task:     task,
		Spans:    spans,
		HasSpans: len(spans) > 0,
		Exception:   exceptionInfo,
		Messages:    messages,
	})
}

var TaskDetailController = taskDetailController{}
