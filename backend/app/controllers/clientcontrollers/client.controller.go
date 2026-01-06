package clientcontrollers

import (
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
)

type clientController struct{}

type ReportRequest struct {
	App              string                          `json:"app"`
	CollectionFrames []*clientmodels.CollectionFrame `json:"collectionFrames"`
}

func (e clientController) Report(c *gin.Context) {
	// Get project ID from context (set by middleware)
	projectId := middleware.GetProjectId(c)

	// we need to parse the request
	var request ReportRequest
	if err := c.ShouldBindBodyWithJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transactionsToInsert := []models.Transaction{}
	exceptionStackTraceToInsert := []models.ExceptionStackTrace{}
	metricRecordsToInsert := []models.MetricRecord{}
	for _, cf := range request.CollectionFrames {
		for _, ct := range cf.Transactions {
			t := ct.ToTransaction()
			t.ProjectId = projectId
			transactionsToInsert = append(transactionsToInsert, t)
		}

		for _, cst := range cf.StackTraces {
			est := cst.ToExceptionStackTrace(computeExceptionHash(cst.StackTrace))
			est.ProjectId = projectId
			exceptionStackTraceToInsert = append(exceptionStackTraceToInsert, est)
		}

		for _, cm := range cf.Metrics {
			mr := cm.ToMetricRecord()
			mr.ProjectId = projectId
			metricRecordsToInsert = append(metricRecordsToInsert, mr)
		}
	}

	err := repositories.TransactionRepository.InsertAsync(c, transactionsToInsert)

	if err != nil {
		panic(err)
	}

	err = repositories.ExceptionStackTraceRepository.InsertAsync(c, exceptionStackTraceToInsert)

	if err != nil {
		panic(err)
	}

	err = repositories.MetricRecordRepository.InsertAsync(c, metricRecordsToInsert)

	if err != nil {
		panic(err)
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

func computeExceptionHash(stackTrace string) string {
	normalized := stackTrace

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

	// Compute SHA-256 and return first 16 hex characters
	normalized = strings.TrimSpace(normalized)
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])[:16]
}

var ClientController = clientController{}
