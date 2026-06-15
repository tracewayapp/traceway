package clientcontrollers

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/models/clientmodels"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/recordings"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	"github.com/tracewayapp/traceway/backend/app/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type clientController struct{}

func isEmptyRaw(r json.RawMessage) bool {
	if len(r) == 0 {
		return true
	}
	trimmed := bytes.TrimSpace(r)
	return bytes.Equal(trimmed, []byte("null")) ||
		bytes.Equal(trimmed, []byte("[]")) ||
		bytes.Equal(trimmed, []byte("{}"))
}

type ReportRequest struct {
	CollectionFrames []*clientmodels.CollectionFrame `json:"collectionFrames"`
	AppVersion       string                          `json:"appVersion"`
	ServerName       string                          `json:"serverName"`
}

func (e clientController) Report(c *gin.Context) {
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

	if project != nil && project.OrganizationId != nil {
		if !hooks.CanReport(*project.OrganizationId) {
			monitoring.RecordRateLimited(*project.OrganizationId)
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
	}

	parseSpan := traceway.StartSpan(c, "report.parse_body")
	var request ReportRequest
	if err := c.ShouldBindBodyWithJSON(&request); err != nil {
		parseSpan.End()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parseSpan.End()

	bodyBytes := 0
	if cb, ok := c.Get(gin.BodyBytesKey); ok {
		if b, ok := cb.([]byte); ok {
			bodyBytes = len(b)
		}
	}

	convertStart := time.Now()

	endpointsToInsert := []models.Endpoint{}
	tasksToInsert := []models.Task{}
	exceptionStackTraceToInsert := []models.ExceptionStackTrace{}
	metricPointsToInsert := []models.MetricPoint{}
	spansToInsert := []models.Span{}
	sessionsToUpsert := []models.Session{}

	var recordingsWork []recordings.Job

	recordingIdToExceptionId := map[string]uuid.UUID{}

	convertSpan := traceway.StartSpan(c, "report.convert_frames")
	for _, cf := range request.CollectionFrames {
		for _, cs := range cf.Sessions {
			s := cs.ToSession(request.AppVersion, request.ServerName)
			s.ProjectId = projectId

			if clientIP := c.ClientIP(); clientIP != "" {
				if s.Attributes == nil {
					s.Attributes = map[string]string{}
				}
				s.Attributes["client.ip"] = clientIP
				s.ClientIP = clientIP
			}
			sessionsToUpsert = append(sessionsToUpsert, s)
		}

		for _, ct := range cf.Traces {
			if ct.IsTask {
				t := ct.ToTask(request.AppVersion, request.ServerName)
				t.ProjectId = projectId
				tasksToInsert = append(tasksToInsert, t)
			} else {
				e := ct.ToEndpoint(request.AppVersion, request.ServerName)
				e.ProjectId = projectId
				if e.StatusCode == 404 {
					e.Endpoint = "UNMATCHED"
				}
				endpointsToInsert = append(endpointsToInsert, e)
			}

			for _, cs := range ct.Spans {
				span := cs.ToSpan(ct.ParsedId())
				span.ProjectId = projectId
				spansToInsert = append(spansToInsert, span)
			}
		}
		resolveJs := project != nil && project.SourceMapToken != nil && jsFrameworks[project.Framework]
		resolveDart := project != nil && project.SourceMapToken != nil && project.Framework == "flutter"

		resolveSpan := traceway.StartSpan(c, "report.resolve_stack_traces")
		for _, cst := range cf.StackTraces {
			resolvedStackTrace := cst.StackTrace
			if resolveJs {
				resolvedStackTrace = services.ResolveStackTrace(c, projectId, cst.StackTrace, cst.DebugIds)
			} else if resolveDart {
				resolvedStackTrace = services.ResolveDartStackTrace(c, projectId, cst.StackTrace)
			}
			est := cst.ToExceptionStackTrace(ComputeExceptionHash(resolvedStackTrace, cst.IsMessage), request.AppVersion, request.ServerName)
			est.StackTrace = resolvedStackTrace
			est.Id = uuid.New()
			est.ProjectId = projectId
			if cst.SessionRecordingId != nil {
				recordingIdToExceptionId[*cst.SessionRecordingId] = est.Id
			}

			if cst.SessionId != nil {
				if parsed, err := uuid.Parse(*cst.SessionId); err == nil {
					est.SessionId = &parsed
				}
			}
			exceptionStackTraceToInsert = append(exceptionStackTraceToInsert, est)
		}
		resolveSpan.End()

		for _, cm := range cf.Metrics {
			mp := cm.ToMetricPoint(request.ServerName)
			mp.ProjectId = projectId
			metricPointsToInsert = append(metricPointsToInsert, mp)
		}

		for _, sr := range cf.SessionRecordings {

			var exceptionId uuid.UUID
			if sr.ExceptionId != "" {
				if id, ok := recordingIdToExceptionId[sr.ExceptionId]; ok {
					exceptionId = id
				}
			}
			var sessionPtr *uuid.UUID
			if sr.SessionId != "" {
				if parsed, err := uuid.Parse(sr.SessionId); err == nil {
					sessionPtr = &parsed
				}
			}
			if exceptionId == uuid.Nil && sessionPtr == nil {
				continue
			}
			if isEmptyRaw(sr.Events) && isEmptyRaw(sr.Logs) && isEmptyRaw(sr.Actions) {
				continue
			}
			body, err := json.Marshal(sr)
			if err != nil {
				traceway.CaptureException(traceway.NewStackTraceErrorf("failed to marshal session recording: %w", err))
				continue
			}
			var key string
			if sessionPtr != nil {
				key = fmt.Sprintf("recordings/%s/sessions/%s/%d.json", projectId, sessionPtr.String(), sr.SegmentIndex)
			} else {
				key = fmt.Sprintf("recordings/%s/%s.json", projectId, exceptionId)
			}
			recordingsWork = append(recordingsWork, recordings.Job{
				Id:           uuid.New(),
				ProjectId:    projectId,
				ExceptionId:  exceptionId,
				SessionId:    sessionPtr,
				SegmentIndex: sr.SegmentIndex,
				Key:          key,
				Body:         body,
				RecordedAt:   time.Now().UTC(),
			})
		}
	}
	convertSpan.End()

	var droppedHealthchecks int
	endpointsToInsert, spansToInsert, droppedHealthchecks = services.FilterHealthchecks(project, endpointsToInsert, spansToInsert, exceptionStackTraceToInsert)
	if droppedHealthchecks > 0 {
		monitoring.RecordHealthchecksDropped(monitoring.SignalNative, droppedHealthchecks)
	}

	convertMs := float64(time.Since(convertStart).Microseconds()) / 1000.0
	insertStart := time.Now()

	if len(endpointsToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.endpoints")
		err := repositories.EndpointRepository.InsertAsync(c, endpointsToInsert)
		insertSpan.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting endpointsToInsert: %w", err))
			return
		}
	}

	if len(tasksToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.tasks")
		err := repositories.TaskRepository.InsertAsync(c, tasksToInsert)
		insertSpan.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting tasksToInsert: %w", err))
			return
		}
	}

	if len(sessionsToUpsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.upsert.sessions")
		err := repositories.SessionRepository.Upsert(c, sessionsToUpsert)
		insertSpan.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error upserting sessions: %w", err))
			return
		}
	}

	exceptionInsertSpan := traceway.StartSpan(c, "report.insert.exceptions")
	err = repositories.ExceptionStackTraceRepository.InsertAsync(c, exceptionStackTraceToInsert)
	exceptionInsertSpan.End()

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting exceptionStackTraceToInsert: %w", err))
		return
	}

	if len(metricPointsToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.metric_points")
		err := repositories.MetricPointRepository.InsertAsync(c, metricPointsToInsert)
		insertSpan.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting metricPointsToInsert: %w", err))
			return
		}

		metricNames := services.CollectUniqueMetricNames(metricPointsToInsert)
		go services.AutoRegisterMetrics(projectId, metricNames)
	}

	spanInsertSpan := traceway.StartSpan(c, "report.insert.spans")
	err = repositories.SpanRepository.InsertAsync(c, spansToInsert)
	spanInsertSpan.End()

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting spansToInsert: %w", err))
		return
	}

	insertMs := float64(time.Since(insertStart).Microseconds()) / 1000.0
	totalSize := len(endpointsToInsert) + len(tasksToInsert) + len(spansToInsert) + len(exceptionStackTraceToInsert) + len(metricPointsToInsert)
	monitoring.RecordIngestBatch(monitoring.SignalNative, "report", convertMs, insertMs, totalSize, bodyBytes)

	var exceptionHashes []string
	for _, est := range exceptionStackTraceToInsert {
		exceptionHashes = append(exceptionHashes, est.ExceptionHash)
	}

	if project != nil && project.OrganizationId != nil {
		hooks.BroadcastReport(hooks.ReportEvent{
			OrganizationId:  *project.OrganizationId,
			ProjectId:       projectId,
			EndpointCount:   len(endpointsToInsert),
			ErrorCount:      len(exceptionStackTraceToInsert),
			TaskCount:       len(tasksToInsert),
			RecordingCount:  len(recordingsWork),
			ExceptionHashes: exceptionHashes,
		})
	}

	for _, rw := range recordingsWork {
		recordings.Enqueue(rw)
	}

	c.JSON(http.StatusOK, gin.H{})
}

var (
	errorMessageRe = regexp.MustCompile(`(?m)^(\*?[\w.]+):\s*.+`)
	causedByRe     = regexp.MustCompile(`(?m)^(Caused by:\s*[\w.$]+):\s*.+`)
	jsFuncLineRe   = regexp.MustCompile(`(?m)^( {0,4})(.+)\(\)(\n {4}.+:\d+:\d+)$`)
	urlOriginRe    = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^/\s]*`)
	absolutePathRe = regexp.MustCompile(`/[^\s:]+/([^/\s:]+:\d+)`)

	laterLineColRe = regexp.MustCompile(`(?m)^(\s*.+:(?:[2-9]|[1-9]\d+)):\d+$`)
	versionRe      = regexp.MustCompile(`@v[\d.]+`)
	hexRe          = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	uuidRe         = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	largeNumberRe  = regexp.MustCompile(`(^|[^:\d])(\d{5,})($|[^\d])`)
	emailRe        = regexp.MustCompile(`[\w.\-]+@[\w.\-]+\.\w+`)
	ipRe           = regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?`)
	goroutineRe    = regexp.MustCompile(`goroutine \d+`)
	javaLineNumRe  = regexp.MustCompile(`\((\w[\w.$]*\.(?:java|kt|scala)):\d+\)`)
	javaEllipsisRe = regexp.MustCompile(`\.\.\. \d+ more`)
	spacesRe       = regexp.MustCompile(`[ \t]+`)
	newlinesRe     = regexp.MustCompile(`\n+`)
)

func ComputeExceptionHash(stackTrace string, isMessage bool) string {
	normalized := stackTrace

	if !isMessage {
		normalized = causedByRe.ReplaceAllString(normalized, "$1")
		normalized = errorMessageRe.ReplaceAllString(normalized, "$1")
		normalized = jsFuncLineRe.ReplaceAllString(normalized, "${1}<fn>${3}")

		normalized = urlOriginRe.ReplaceAllString(normalized, "")
		normalized = absolutePathRe.ReplaceAllString(normalized, "$1")
		normalized = laterLineColRe.ReplaceAllString(normalized, "$1")
		normalized = versionRe.ReplaceAllString(normalized, "")
		normalized = hexRe.ReplaceAllString(normalized, "<hex>")
		normalized = uuidRe.ReplaceAllString(normalized, "<uuid>")
		normalized = largeNumberRe.ReplaceAllString(normalized, "${1}<id>${3}")
		normalized = emailRe.ReplaceAllString(normalized, "<email>")
		normalized = ipRe.ReplaceAllString(normalized, "<ip>")
		normalized = goroutineRe.ReplaceAllString(normalized, "goroutine <n>")
		normalized = javaLineNumRe.ReplaceAllString(normalized, "($1)")
		normalized = javaEllipsisRe.ReplaceAllString(normalized, "... more")
		normalized = spacesRe.ReplaceAllString(normalized, " ")
		normalized = newlinesRe.ReplaceAllString(normalized, "\n")
	}

	normalized = strings.TrimSpace(normalized)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:16]
}

var jsFrameworks = map[string]bool{
	"react":        true,
	"svelte":       true,
	"vuejs":        true,
	"jquery":       true,
	"nextjs":       true,
	"nestjs":       true,
	"express":      true,
	"remix":        true,
	"react-native": true,
}

var frontendJsFrameworks = map[string]bool{
	"react":        true,
	"svelte":       true,
	"vuejs":        true,
	"jquery":       true,
	"react-native": true,
}

func IsFrontendFramework(framework string) bool {
	return frontendJsFrameworks[framework]
}

var ClientController = clientController{}
