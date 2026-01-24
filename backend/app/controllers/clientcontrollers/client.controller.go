package clientcontrollers

import (
	"backend/app/hooks"
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/models/clientmodels"
	"backend/app/repositories"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"

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

	var request ReportRequest
	if err := c.ShouldBindBodyWithJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	endpointsToInsert := []models.Endpoint{}
	tasksToInsert := []models.Task{}
	exceptionStackTraceToInsert := []models.ExceptionStackTrace{}
	metricRecordsToInsert := []models.MetricRecord{}
	segmentsToInsert := []models.Segment{}
	for _, cf := range request.CollectionFrames {
		for _, ct := range cf.Transactions {
			if ct.IsTask {
				t := ct.ToTask(request.AppVersion, request.ServerName)
				t.ProjectId = projectId
				tasksToInsert = append(tasksToInsert, t)
			} else {
				e := ct.ToEndpoint(request.AppVersion, request.ServerName)
				e.ProjectId = projectId
				endpointsToInsert = append(endpointsToInsert, e)
			}

			// Extract segments from transaction
			for _, cs := range ct.Segments {
				seg := cs.ToSegment(ct.ParsedId())
				seg.ProjectId = projectId
				segmentsToInsert = append(segmentsToInsert, seg)
			}
		}

		for _, cst := range cf.StackTraces {
			est := cst.ToExceptionStackTrace(computeExceptionHash(cst.StackTrace, cst.IsMessage), request.AppVersion, request.ServerName)
			est.Id = uuid.New()
			est.ProjectId = projectId
			exceptionStackTraceToInsert = append(exceptionStackTraceToInsert, est)
		}

		for _, cm := range cf.Metrics {
			mr := cm.ToMetricRecord(request.ServerName)
			mr.ProjectId = projectId
			metricRecordsToInsert = append(metricRecordsToInsert, mr)
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

	err = repositories.SegmentRepository.InsertAsync(c, segmentsToInsert)

	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error inserting segmentsToInsert: %w", err))
		return
	}

	// Broadcast usage event
	if project, exists := c.Get(middleware.ProjectContextKey); exists {
		if p, ok := project.(*models.Project); ok && p.OrganizationId != nil {
			hooks.BroadcastReport(hooks.ReportEvent{
				OrganizationId: *p.OrganizationId,
				EndpointCount:  len(endpointsToInsert),
				ErrorCount:     len(exceptionStackTraceToInsert),
				TaskCount:      len(tasksToInsert),
			})
		}
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
		// Only normalize for actual exceptions, not messages
		// Remove the error message content (keep just the error type)
		normalized = errorMessageRe.ReplaceAllString(normalized, "$1")

		// Remove absolute paths, keep just filename:line
		normalized = absolutePathRe.ReplaceAllString(normalized, "$1")

		// Remove version numbers from module paths
		normalized = versionRe.ReplaceAllString(normalized, "")

		// Replace hex addresses/pointers
		normalized = hexRe.ReplaceAllString(normalized, "<hex>")

		// Replace UUIDs
		normalized = uuidRe.ReplaceAllString(normalized, "<uuid>")

		// Replace standalone large numbers (likely IDs, not line numbers)
		// Since Go doesn't support lookbehind, we preserve the surrounding characters
		normalized = largeNumberRe.ReplaceAllString(normalized, "${1}<id>${3}")

		// Replace email addresses
		normalized = emailRe.ReplaceAllString(normalized, "<email>")

		// Replace IP addresses
		normalized = ipRe.ReplaceAllString(normalized, "<ip>")

		// Normalize goroutine numbers
		normalized = goroutineRe.ReplaceAllString(normalized, "goroutine <n>")

		// Normalize whitespace
		normalized = spacesRe.ReplaceAllString(normalized, " ")
		normalized = newlinesRe.ReplaceAllString(normalized, "\n")
	}

	// Compute SHA-256 and return first 16 hex characters
	normalized = strings.TrimSpace(normalized)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:16]
}

var ClientController = clientController{}
