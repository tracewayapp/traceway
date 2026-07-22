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
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/app/services"
	"github.com/tracewayapp/traceway/backend/app/symbolicator/sourcemap/jsstack"

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

func symbolicateRecordingErrorLogs(c *gin.Context, projectId uuid.UUID, raw json.RawMessage) json.RawMessage {
	var logs []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &logs); err != nil {
		return raw
	}
	changed := false
	for _, entry := range logs {
		var level string
		if err := json.Unmarshal(entry["level"], &level); err != nil || !strings.EqualFold(level, "error") {
			continue
		}
		var message string
		if err := json.Unmarshal(entry["message"], &message); err != nil {
			continue
		}
		canonical, ok := jsstack.Canonicalize(message)
		if !ok {
			continue
		}
		resolved := services.ResolveStackTrace(c, projectId, canonical, nil)
		if resolved == canonical {
			continue
		}
		encoded, err := json.Marshal(resolved)
		if err != nil {
			continue
		}
		entry["message"] = encoded
		changed = true
	}
	if !changed {
		return raw
	}
	out, err := json.Marshal(logs)
	if err != nil {
		return raw
	}
	return out
}

type ReportRequest struct {
	CollectionFrames []*clientmodels.CollectionFrame `json:"collectionFrames"`
	AppVersion       string                          `json:"appVersion"`
	ServerName       string                          `json:"serverName"`
	ProguardUuid     string                          `json:"proguardUuid"`
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

	orgId := 0
	hasOrg := project != nil && project.OrganizationId != nil
	if hasOrg {
		orgId = *project.OrganizationId
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
		resolveIos := project != nil && project.SourceMapToken != nil && project.Framework == "ios"
		resolveAndroid := project != nil && project.SourceMapToken != nil && project.Framework == "android"

		resolveSpan := traceway.StartSpan(c, "report.resolve_stack_traces")
		for _, cst := range cf.StackTraces {
			resolvedStackTrace := cst.StackTrace
			if resolveJs {
				resolvedStackTrace = services.ResolveStackTrace(c, projectId, cst.StackTrace, cst.DebugIds)
			} else if resolveDart {
				resolvedStackTrace = services.ResolveDartStackTrace(c, projectId, cst.StackTrace)
			} else if resolveIos {
				resolvedStackTrace = services.ResolveIOSStackTrace(c, projectId, cst.StackTrace)
			} else if resolveAndroid {
				resolvedStackTrace = services.ResolveAndroidStackTrace(c, projectId, cst.StackTrace, request.ProguardUuid)
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
			if resolveJs && !isEmptyRaw(sr.Logs) {
				sr.Logs = symbolicateRecordingErrorLogs(c, projectId, sr.Logs)
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

	perm := hooks.IngestPermission{Exceptions: true, Data: true, Replay: true}
	if hasOrg {
		perm = hooks.IngestPermissionFor(orgId)
	}

	recordingBytes := 0
	for _, rw := range recordingsWork {
		recordingBytes += len(rw.Body)
	}
	telemetryBytes := bodyBytes - recordingBytes
	if telemetryBytes < 0 {
		telemetryBytes = 0
	}

	if !perm.Exceptions {
		exceptionStackTraceToInsert = nil
	}
	if !perm.Data {
		endpointsToInsert = nil
		tasksToInsert = nil
		spansToInsert = nil
		metricPointsToInsert = nil
	}
	if !perm.Replay {
		recordingsWork = nil
	}
	if hasOrg && (!perm.Exceptions || !perm.Data) {
		monitoring.RecordRateLimited(orgId)
	}

	convertMs := float64(time.Since(convertStart).Microseconds()) / 1000.0
	insertStart := time.Now()

	if len(endpointsToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.endpoints")
		err := telemetry.EndpointRepository.InsertAsync(c, endpointsToInsert)
		insertSpan.End()
		if err != nil {
			abortIngestInsertError(c, err, "endpointsToInsert")
			return
		}
	}

	if len(tasksToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.tasks")
		err := telemetry.TaskRepository.InsertAsync(c, tasksToInsert)
		insertSpan.End()
		if err != nil {
			abortIngestInsertError(c, err, "tasksToInsert")
			return
		}
	}

	if len(sessionsToUpsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.upsert.sessions")
		err := telemetry.SessionRepository.Upsert(c, sessionsToUpsert)
		insertSpan.End()
		if err != nil {
			c.AbortWithError(500, traceway.NewStackTraceErrorf("error upserting sessions: %w", err))
			return
		}
	}

	exceptionInsertSpan := traceway.StartSpan(c, "report.insert.exceptions")
	err = telemetry.ExceptionStackTraceRepository.InsertAsync(c, exceptionStackTraceToInsert)
	exceptionInsertSpan.End()

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting exceptionStackTraceToInsert: %w", err))
		return
	}

	if len(metricPointsToInsert) > 0 {
		insertSpan := traceway.StartSpan(c, "report.insert.metric_points")
		err := telemetry.MetricPointRepository.InsertAsync(c, metricPointsToInsert)
		insertSpan.End()
		if err != nil {
			abortIngestInsertError(c, err, "metricPointsToInsert")
			return
		}

		metricNames := services.CollectUniqueMetricNames(metricPointsToInsert)
		go services.AutoRegisterMetrics(projectId, metricNames)
	}

	spanInsertSpan := traceway.StartSpan(c, "report.insert.spans")
	err = telemetry.SpanRepository.InsertAsync(c, spansToInsert)
	spanInsertSpan.End()

	if err != nil {
		abortIngestInsertError(c, err, "spansToInsert")
		return
	}

	insertMs := float64(time.Since(insertStart).Microseconds()) / 1000.0
	totalSize := len(endpointsToInsert) + len(tasksToInsert) + len(spansToInsert) + len(exceptionStackTraceToInsert) + len(metricPointsToInsert)
	monitoring.RecordIngestBatch(monitoring.SignalNative, "report", convertMs, insertMs, totalSize, bodyBytes)

	var exceptionHashes []string
	for _, est := range exceptionStackTraceToInsert {
		exceptionHashes = append(exceptionHashes, est.ExceptionHash)
	}

	if hasOrg {
		ev := hooks.ReportEvent{
			OrganizationId: orgId,
			ProjectId:      projectId,
		}
		if perm.Data {
			ev.EndpointCount = len(endpointsToInsert)
			ev.TaskCount = len(tasksToInsert)
			ev.TelemetryBytes = telemetryBytes
		}
		if perm.Exceptions {
			ev.ErrorCount = len(exceptionStackTraceToInsert)
			ev.ExceptionHashes = exceptionHashes
		}
		if perm.Replay {
			ev.RecordingCount = len(recordingsWork)
			ev.RecordingBytes = recordingBytes
		}
		hooks.BroadcastReport(ev)
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
