package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"

	"github.com/gin-gonic/gin"
)

type taskDetailController struct{}

type TaskDetailRequest struct {
	ProjectId string `json:"projectId"`
}

type TaskExceptionInfo struct {
	ExceptionHash string `json:"exceptionHash"`
	StackTrace    string `json:"stackTrace"`
	RecordedAt    string `json:"recordedAt"`
}

type TaskDetailResponse struct {
	Task        *models.Task       `json:"task"`
	Segments    []models.Segment   `json:"segments"`
	HasSegments bool               `json:"hasSegments"`
	Exception   *TaskExceptionInfo `json:"exception,omitempty"`
}

func (c taskDetailController) GetTaskDetail(ctx *gin.Context) {
	taskId := ctx.Param("taskId")
	if taskId == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "taskId is required"})
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

	// Get linked exception if any
	var exceptionInfo *TaskExceptionInfo
	exception, err := repositories.ExceptionStackTraceRepository.FindByTransactionId(ctx, request.ProjectId, taskId)
	if err != nil {
		panic(err)
	}
	if exception != nil {
		exceptionInfo = &TaskExceptionInfo{
			ExceptionHash: exception.ExceptionHash,
			StackTrace:    exception.StackTrace,
			RecordedAt:    exception.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	ctx.JSON(http.StatusOK, TaskDetailResponse{
		Task:        task,
		Segments:    segments,
		HasSegments: len(segments) > 0,
		Exception:   exceptionInfo,
	})
}

var TaskDetailController = taskDetailController{}
