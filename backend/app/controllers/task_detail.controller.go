package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type taskDetailController struct{}

type TaskDetailRequest struct {
	ProjectId uuid.UUID `json:"projectId"`
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
	Scope         map[string]string `json:"scope,omitempty"`
}

type TaskDetailResponse struct {
	Task        *models.Task       `json:"task"`
	Segments    []models.Segment   `json:"segments"`
	HasSegments bool               `json:"hasSegments"`
	Exception   *TaskExceptionInfo `json:"exception,omitempty"`
	Messages    []TaskMessageInfo  `json:"messages"`
}

func (c taskDetailController) GetTaskDetail(ctx *gin.Context) {
	taskId, err := uuid.Parse(ctx.Param("taskId"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid taskId"})
		return
	}

	var request TaskDetailRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get task
	task, err := repositories.TaskRepository.FindById(ctx, request.ProjectId, taskId)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Get segments (flat list ordered by start_time)
	segments, err := repositories.SegmentRepository.FindByTransactionId(ctx, request.ProjectId, taskId)
	if err != nil {
		panic(err)
	}

	// Get all linked exceptions and messages
	var exceptionInfo *TaskExceptionInfo
	var messages []TaskMessageInfo

	allExceptions, err := repositories.ExceptionStackTraceRepository.FindAllByTransactionId(ctx, request.ProjectId, taskId)
	if err != nil {
		panic(err)
	}

	for _, exc := range allExceptions {
		if exc.IsMessage {
			// Add to messages list
			messages = append(messages, TaskMessageInfo{
				Id:            exc.Id,
				ExceptionHash: exc.ExceptionHash,
				StackTrace:    exc.StackTrace,
				RecordedAt:    exc.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
				Scope:         exc.Scope,
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

	ctx.JSON(http.StatusOK, TaskDetailResponse{
		Task:        task,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
		Messages:    messages,
	})
}

var TaskDetailController = taskDetailController{}
