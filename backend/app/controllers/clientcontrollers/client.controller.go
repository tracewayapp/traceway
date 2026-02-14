package clientcontrollers

import (
	"backend/app/hooks"
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/models/clientmodels"
	"backend/app/repositories"
	"backend/app/storage"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type clientController struct{}

type ReportRequest struct {
	CollectionFrames []*clientmodels.CollectionFrame `json:"collectionFrames"`
	AppVersion       string                          `json:"appVersion"`
	ServerName       string                          `json:"serverName"`
}

func (e clientController) Report(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("UseClientAuth middleware must be applied: %w", err))
		return
	}

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			if !hooks.CanReport(*p.OrganizationId) {
				c.AbortWithStatus(http.StatusTooManyRequests)
				return
			}
		}
	}

	var request ReportRequest
	if err := c.ShouldBindBodyWithJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	endpointsToInsert := []models.Endpoint{}
	tasksToInsert := []models.Task{}
	exceptionStackTraceToInsert := []models.ExceptionStackTrace{}
	metricRecordsToInsert := []models.MetricRecord{}
	spansToInsert := []models.Span{}

	type recordingWork struct {
		Id          uuid.UUID
		ProjectId   uuid.UUID
		ExceptionId uuid.UUID
		Events      []byte
		RecordedAt  time.Time
	}
	var recordingsWork []recordingWork

	// Map frontend sessionRecordingId → backend-generated exception UUID
	recordingIdToExceptionId := map[string]uuid.UUID{}

	for _, cf := range request.CollectionFrames {
		for _, ct := range cf.Traces {
			if ct.IsTask {
				t := ct.ToTask(request.AppVersion, request.ServerName)
				t.ProjectId = projectId
				tasksToInsert = append(tasksToInsert, t)
			} else {
				e := ct.ToEndpoint(request.AppVersion, request.ServerName)
				e.ProjectId = projectId
				endpointsToInsert = append(endpointsToInsert, e)
			}

			for _, cs := range ct.Spans {
				span := cs.ToSpan(ct.ParsedId())
				span.ProjectId = projectId
				spansToInsert = append(spansToInsert, span)
			}
		}

		for _, cst := range cf.StackTraces {
			est := cst.ToExceptionStackTrace(computeExceptionHash(cst.StackTrace, cst.IsMessage), request.AppVersion, request.ServerName)
			est.Id = uuid.New()
			est.ProjectId = projectId
			if cst.SessionRecordingId != nil {
				recordingIdToExceptionId[*cst.SessionRecordingId] = est.Id
			}
			exceptionStackTraceToInsert = append(exceptionStackTraceToInsert, est)
		}

		for _, cm := range cf.Metrics {
			mr := cm.ToMetricRecord(request.ServerName)
			mr.ProjectId = projectId
			metricRecordsToInsert = append(metricRecordsToInsert, mr)
		}

		for _, sr := range cf.SessionRecordings {
			exceptionId, ok := recordingIdToExceptionId[sr.ExceptionId]
			if !ok {
				continue
			}
			recordingsWork = append(recordingsWork, recordingWork{
				Id:          uuid.New(),
				ProjectId:   projectId,
				ExceptionId: exceptionId,
				Events:      sr.Events,
				RecordedAt:  time.Now().UTC(),
			})
		}
	}

	if len(endpointsToInsert) > 0 {
		err := repositories.EndpointRepository.InsertAsync(c, endpointsToInsert)
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting endpointsToInsert: %w", err))
			return
		}
	}

	if len(tasksToInsert) > 0 {
		err := repositories.TaskRepository.InsertAsync(c, tasksToInsert)
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting tasksToInsert: %w", err))
			return
		}
	}

	err = repositories.ExceptionStackTraceRepository.InsertAsync(c, exceptionStackTraceToInsert)

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting exceptionStackTraceToInsert: %w", err))
		return
	}

	err = repositories.MetricRecordRepository.InsertAsync(c, metricRecordsToInsert)

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting metricRecordsToInsert: %w", err))
		return
	}

	err = repositories.SpanRepository.InsertAsync(c, spansToInsert)

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting spansToInsert: %w", err))
		return
	}

	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			hooks.BroadcastReport(hooks.ReportEvent{
				OrganizationId: *p.OrganizationId,
				EndpointCount:  len(endpointsToInsert),
				ErrorCount:     len(exceptionStackTraceToInsert),
				TaskCount:      len(tasksToInsert),
				RecordingCount: len(recordingsWork),
			})
		}
	}

	if len(recordingsWork) > 0 {
		work := recordingsWork
		go func() {
			var successful []models.SessionRecording
			for _, rw := range work {
				key := fmt.Sprintf("recordings/%s/%s.json", rw.ProjectId, rw.ExceptionId)
				if err := storage.Store.Write(context.Background(), key, rw.Events); err != nil {
					traceway.CaptureException(traceway.NewStackTraceErrorf("failed to write session recording (key=%s): %w", key, err))
					continue
				}
				successful = append(successful, models.SessionRecording{
					Id:          rw.Id,
					ProjectId:   rw.ProjectId,
					ExceptionId: rw.ExceptionId,
					FilePath:    key,
					RecordedAt:  rw.RecordedAt,
				})
			}
			if len(successful) > 0 {
				if err := repositories.SessionRecordingRepository.InsertAsync(context.Background(), successful); err != nil {
					traceway.CaptureException(traceway.NewStackTraceErrorf("failed to batch insert %d session recording refs: %w", len(successful), err))
				}
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{})
}

var (
	errorMessageRe = regexp.MustCompile(`(?m)^(\*?[\w.]+):\s*.+`)
	absolutePathRe = regexp.MustCompile(`/[^\s:]+/([^/\s:]+:\d+)`)
	versionRe      = regexp.MustCompile(`@v[\d.]+`)
	hexRe          = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	uuidRe         = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	largeNumberRe  = regexp.MustCompile(`(^|[^:\d])(\d{5,})($|[^\d])`)
	emailRe        = regexp.MustCompile(`[\w.\-]+@[\w.\-]+\.\w+`)
	ipRe           = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?`)
	goroutineRe    = regexp.MustCompile(`goroutine \d+`)
	spacesRe       = regexp.MustCompile(`[ \t]+`)
	newlinesRe     = regexp.MustCompile(`\n+`)
)

func computeExceptionHash(stackTrace string, isMessage bool) string {
	normalized := stackTrace

	if !isMessage {
		normalized = errorMessageRe.ReplaceAllString(normalized, "$1")
		normalized = absolutePathRe.ReplaceAllString(normalized, "$1")
		normalized = versionRe.ReplaceAllString(normalized, "")
		normalized = hexRe.ReplaceAllString(normalized, "<hex>")
		normalized = uuidRe.ReplaceAllString(normalized, "<uuid>")
		normalized = largeNumberRe.ReplaceAllString(normalized, "${1}<id>${3}")
		normalized = emailRe.ReplaceAllString(normalized, "<email>")
		normalized = ipRe.ReplaceAllString(normalized, "<ip>")
		normalized = goroutineRe.ReplaceAllString(normalized, "goroutine <n>")
		normalized = spacesRe.ReplaceAllString(normalized, " ")
		normalized = newlinesRe.ReplaceAllString(normalized, "\n")
	}

	normalized = strings.TrimSpace(normalized)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:16]
}

var ClientController = clientController{}
