package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type sessionDetailController struct{}

type sessionDetailRequest struct {
	StartedAt *time.Time `json:"startedAt"`
}

type SessionExceptionInfo struct {
	Id            uuid.UUID `json:"id"`
	ExceptionHash string    `json:"exceptionHash"`
	StackTrace    string    `json:"stackTrace"`
	RecordedAt    string    `json:"recordedAt"`
	IsMessage     bool      `json:"isMessage"`
}

type SessionDetailResponse struct {
	Session    *models.Session        `json:"session"`
	Exceptions []SessionExceptionInfo `json:"exceptions"`
}

// SessionRecordingPayload is the shape consumed by the frontend SessionReplay
// component. We concatenate rrweb event arrays from every segment ordered by
// segment_index, plus union the optional logs/actions arrays. Logs/actions are
// kept as raw JSON since the backend never inspects them.
type SessionRecordingPayload struct {
	Events    []json.RawMessage `json:"events"`
	Logs      []json.RawMessage `json:"logs,omitempty"`
	Actions   []json.RawMessage `json:"actions,omitempty"`
	StartedAt *time.Time        `json:"startedAt,omitempty"`
	EndedAt   *time.Time        `json:"endedAt,omitempty"`
}

func (s sessionDetailController) GetSessionDetail(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	sessionId, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sessionId"})
		return
	}

	var request sessionDetailRequest
	_ = c.ShouldBindJSON(&request)

	span := traceway.StartSpan(c, "loading session")
	session, err := telemetry.SessionRepository.FindById(c, projectId, sessionId, request.StartedAt)
	if session == nil && err == nil && request.StartedAt != nil {
		session, err = telemetry.SessionRepository.FindById(c, projectId, sessionId, nil)
	}
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading session: %w", err))
		return
	}
	if session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	span = traceway.StartSpan(c, "loading session exceptions")
	exceptions, err := telemetry.ExceptionStackTraceRepository.FindAllBySessionId(c, projectId, sessionId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading session exceptions: %w", err))
		return
	}

	out := make([]SessionExceptionInfo, 0, len(exceptions))
	for _, e := range exceptions {
		out = append(out, SessionExceptionInfo{
			Id:            e.Id,
			ExceptionHash: e.ExceptionHash,
			StackTrace:    e.StackTrace,
			RecordedAt:    e.RecordedAt.Format("2006-01-02T15:04:05Z07:00"),
			IsMessage:     e.IsMessage,
		})
	}

	c.JSON(http.StatusOK, SessionDetailResponse{
		Session:    session,
		Exceptions: out,
	})
}

func (s sessionDetailController) GetSessionRecording(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	sessionId, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid sessionId"})
		return
	}

	span := traceway.StartSpan(c, "loading session segments")
	segments, err := telemetry.SessionRecordingRepository.FindBySessionId(c, projectId, sessionId)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading session segments: %w", err))
		return
	}

	payload := SessionRecordingPayload{
		Events: []json.RawMessage{},
	}

	type segmentBody struct {
		Events    json.RawMessage `json:"events"`
		Logs      json.RawMessage `json:"logs"`
		Actions   json.RawMessage `json:"actions"`
		StartedAt *time.Time      `json:"startedAt"`
		EndedAt   *time.Time      `json:"endedAt"`
	}

	for _, seg := range segments {
		raw, err := storage.Store.Read(context.Background(), seg.FilePath)
		if err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to read session segment (key=%s): %w", seg.FilePath, err))
			continue
		}
		var body segmentBody
		if err := json.Unmarshal(raw, &body); err != nil {
			traceway.CaptureException(traceway.NewStackTraceErrorf("failed to parse session segment (key=%s): %w", seg.FilePath, err))
			continue
		}

		if len(body.Events) > 0 {
			var events []json.RawMessage
			if err := json.Unmarshal(body.Events, &events); err == nil {
				payload.Events = append(payload.Events, events...)
			}
		}
		if len(body.Logs) > 0 {
			var logs []json.RawMessage
			if err := json.Unmarshal(body.Logs, &logs); err == nil {
				payload.Logs = append(payload.Logs, logs...)
			}
		}
		if len(body.Actions) > 0 {
			var actions []json.RawMessage
			if err := json.Unmarshal(body.Actions, &actions); err == nil {
				payload.Actions = append(payload.Actions, actions...)
			}
		}
		if body.StartedAt != nil && (payload.StartedAt == nil || body.StartedAt.Before(*payload.StartedAt)) {
			t := *body.StartedAt
			payload.StartedAt = &t
		}
		if body.EndedAt != nil && (payload.EndedAt == nil || body.EndedAt.After(*payload.EndedAt)) {
			t := *body.EndedAt
			payload.EndedAt = &t
		}
	}

	c.JSON(http.StatusOK, payload)
}

var SessionDetailController = sessionDetailController{}
