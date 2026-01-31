package controllers

import (
	"backend/app/middleware"
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	traceway "go.tracewayapp.com"
)

type metricRecordController struct{}

func (e metricRecordController) FindHomepageStats(c *gin.Context) {
	projectId, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}

	// requests in last 24h vs previous 24h
	now := time.Now()
	oneDayAgo := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	// requests
	span := traceway.StartSpan(c, "loading requests stats")
	requestsNow, err := repositories.EndpointRepository.CountBetween(c, projectId, oneDayAgo, now)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading requestsNow: %w", err))
		return
	}
	requestsPrev, err := repositories.EndpointRepository.CountBetween(c, projectId, twoDaysAgo, oneDayAgo)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading requestsPrev: %w", err))
		return
	}

	// exceptions
	span = traceway.StartSpan(c, "loading exceptions stats")
	exceptionsNow, err := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, oneDayAgo, now)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptionsNow: %w", err))
		return
	}
	exceptionsPrev, err := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, twoDaysAgo, oneDayAgo)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptionsPrev: %w", err))
		return
	}

	// ram usage last 24h vs previous 24h
	span = traceway.StartSpan(c, "loading ram usage")
	ramNow, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, oneDayAgo, now)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ramNow: %w", err))
		return
	}
	ramPrev, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, twoDaysAgo, oneDayAgo)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading ramPrev: %w", err))
		return
	}

	// memory usage last 24h vs previous 24h
	span = traceway.StartSpan(c, "loading cpu usage")
	cpuNow, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, oneDayAgo, now)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading cpuNow: %w", err))
		return
	}
	cpuPrev, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, twoDaysAgo, oneDayAgo)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading cpuPrev: %w", err))
		return
	}

	c.JSON(http.StatusOK, HomepageStatsResponse{
		Requests: StatsComparison{
			Current:       float64(requestsNow),
			Previous:      float64(requestsPrev),
			PercentChange: calculatePercentChange(float64(requestsNow), float64(requestsPrev)),
		},
		Exceptions: StatsComparison{
			Current:       float64(exceptionsNow),
			Previous:      float64(exceptionsPrev),
			PercentChange: calculatePercentChange(float64(exceptionsNow), float64(exceptionsPrev)),
		},
		MemoryUsage: StatsComparison{
			Current:       ramNow,
			Previous:      ramPrev,
			PercentChange: calculatePercentChange(ramNow, ramPrev),
		},
		CpuUsage: StatsComparison{
			Current:       cpuNow,
			Previous:      cpuPrev,
			PercentChange: calculatePercentChange(cpuNow, cpuPrev),
		},
	})
}

type HomepageStatsResponse struct {
	Requests    StatsComparison `json:"requests"`
	Exceptions  StatsComparison `json:"exceptions"`
	MemoryUsage StatsComparison `json:"memoryUsage"`
	CpuUsage    StatsComparison `json:"cpuUsage"`
}

type StatsComparison struct {
	Current       float64 `json:"current"`
	Previous      float64 `json:"previous"`
	PercentChange float64 `json:"percentChange"`
}

func calculatePercentChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return ((current - previous) / previous) * 100
}

var MetricRecordController = metricRecordController{}
