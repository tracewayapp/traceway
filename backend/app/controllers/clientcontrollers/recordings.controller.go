package clientcontrollers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/models/clientmodels"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/recordings"
	traceway "go.tracewayapp.com"
)

const maxRecordingSegments = 100

// RecordingsRequest is the body of the replay endpoint the OpenTelemetry SDKs use. A segment's exceptionId is the
// id the SDK gave the exception (traceway.exception.id), which is also the id the exception is stored under, so a
// clip and its exception may arrive in either order on separate requests.
type RecordingsRequest struct {
	DebugIds map[string]string                      `json:"debugIds,omitempty"`
	Segments []*clientmodels.ClientSessionRecording `json:"segments"`
}

type recordingsController struct{}

var enqueueRecording = recordings.Enqueue

var RecordingsController = recordingsController{}

func (recordingsController) Ingest(c *gin.Context) {
	monitoring.IngestStarted()
	defer monitoring.IngestFinished()

	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}
	var project *models.Project
	if projectAsAny, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := projectAsAny.(*models.Project); ok {
			project = p
		}
	}
	orgId := 0
	hasOrg := project != nil && project.OrganizationId != nil
	if hasOrg {
		orgId = *project.OrganizationId
	}

	var request RecordingsRequest
	if err := c.ShouldBindBodyWithJSON(&request); err != nil {
		middleware.RejectIngestBindError(c, err, err.Error())
		return
	}
	if len(request.Segments) == 0 || len(request.Segments) > maxRecordingSegments {
		c.JSON(http.StatusBadRequest, gin.H{"error": "segments must hold between 1 and 100 recording segments"})
		return
	}

	perm := hooks.IngestPermission{Exceptions: true, Data: true, Replay: true}
	if hasOrg {
		perm = hooks.IngestPermissionFor(orgId)
	}
	if !perm.Replay {
		monitoring.RecordRateLimited(orgId)
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	resolveJs := project != nil && project.SourceMapToken != nil && jsFrameworks[project.Framework]
	var jobs []recordings.Job
	recordingBytes := 0
	for _, sr := range request.Segments {
		if sr == nil {
			continue
		}
		var exceptionId uuid.UUID
		if parsed, err := uuid.Parse(sr.ExceptionId); err == nil {
			exceptionId = parsed
		}
		job, ok := buildRecordingJob(c, projectId, sr, exceptionId, resolveJs, request.DebugIds)
		if !ok {
			continue
		}
		// Without exceptions on the plan, a clip of one can never be reached unless it is also part of a session.
		if !perm.Exceptions && job.SessionId == nil {
			continue
		}
		jobs = append(jobs, job)
		recordingBytes += len(job.Body)
	}

	if hasOrg {
		hooks.BroadcastReport(hooks.ReportEvent{
			OrganizationId: orgId,
			ProjectId:      projectId,
			RecordingCount: len(jobs),
			RecordingBytes: recordingBytes,
		})
	}
	for _, job := range jobs {
		enqueueRecording(job)
	}
	c.JSON(http.StatusOK, gin.H{})
}
