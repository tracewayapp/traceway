package clientcontrollers

import (
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/hooks"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/profiling"
	"github.com/tracewayapp/traceway/backend/app/repositories"
	traceway "go.tracewayapp.com"
)

type profileIngestController struct{}

func (e profileIngestController) Ingest(c *gin.Context) {
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

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	serviceName := c.Query("service")
	if serviceName == "" && project != nil {
		serviceName = project.Name
	}

	ingestCtx := profiling.IngestContext{
		ProjectId:          projectId,
		DefaultServiceName: serviceName,
		ServerName:         c.Query("serverName"),
		AppVersion:         c.Query("appVersion"),
		ReceivedAt:         time.Now().UTC(),
	}

	decoded, err := (profiling.PprofDecoder{}).Decode(ingestCtx, body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convertStart := time.Now()
	var stacks []models.ProfileStack
	var samples []models.ProfileSample
	var profiles []models.Profile
	for _, d := range decoded {
		s, sm, p := profiling.BuildRows(d, projectId)
		stacks = append(stacks, s...)
		samples = append(samples, sm...)
		profiles = append(profiles, p...)
	}
	convertMs := float64(time.Since(convertStart).Microseconds()) / 1000.0

	insertStart := time.Now()
	if err := repositories.ProfileRepository.InsertStacksAsync(c, stacks); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error inserting profiling stacks: %w", err))
		return
	}
	if err := repositories.ProfileRepository.InsertSamplesAsync(c, samples); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error inserting profiling samples: %w", err))
		return
	}
	if err := repositories.ProfileRepository.InsertProfilesAsync(c, profiles); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error inserting profiles: %w", err))
		return
	}
	insertMs := float64(time.Since(insertStart).Microseconds()) / 1000.0

	monitoring.RecordIngestBatch(monitoring.SignalNative, "profiles", convertMs, insertMs, len(samples), len(body))

	c.JSON(http.StatusOK, gin.H{})
}

var ProfileIngestController = profileIngestController{}
