package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type metricsController struct{}

// Response types for split endpoints
type ApplicationMetricsResponse struct {
	Metrics          []models.DashboardMetric `json:"metrics"`
	AvailableServers []string                 `json:"availableServers"`
	LastUpdated      time.Time                `json:"lastUpdated"`
}

type StatsMetricsResponse struct {
	Metrics     []models.DashboardMetric `json:"metrics"`
	LastUpdated time.Time                `json:"lastUpdated"`
}

type ServerMetricsResponse struct {
	Metrics          []models.DashboardMetric `json:"metrics"`
	AvailableServers []string                 `json:"availableServers"`
	LastUpdated      time.Time                `json:"lastUpdated"`
}

// GetApplicationMetrics returns Go application metrics (Go Routines, Heap Objects, GC Cycles, GC Pause)
// Always returns ALL servers' data - ignores server selector
func (m metricsController) GetApplicationMetrics(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	now := time.Now()
	start, end := parseTimeRange(c, now)

	// Calculate previous period for comparison
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	// Calculate aggregation interval based on time range
	intervalMinutes := calculateIntervalMinutes(duration)

	// Get available servers in the time range
	availableServers, err := repositories.MetricRecordRepository.GetDistinctServers(c, projectId, start, end)
	if err != nil {
		availableServers = []string{}
	}

	// Always pass empty slice to get ALL servers
	var emptyServers []string

	metrics := make([]models.DashboardMetric, 0, 4)

	// 1. Go Routines
	seg := traceway.StartSegment(c, "loading go routines")
	goRoutinesPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameGoRoutines, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading goRoutinesPerServer: %w", err))
		return
	}
	goRoutinesPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameGoRoutines, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("go_routines", "Go Routines", "", goRoutinesPerServer, goRoutinesPrev, "go_routines"))

	// 2. Heap Objects
	seg = traceway.StartSegment(c, "loading heap objects")
	heapObjectsPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameHeapObjects, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading heapObjectsPerServer: %w", err))
		return
	}
	heapObjectsPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameHeapObjects, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("heap_objects", "Heap Objects", "", heapObjectsPerServer, heapObjectsPrev, "heap_objects"))

	// 3. GC Cycles (Num GC)
	seg = traceway.StartSegment(c, "loading gc cycles")
	numGCPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameNumGC, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading numGCPerServer: %w", err))
		return
	}
	numGCPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameNumGC, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("num_gc", "GC Cycles", "", numGCPerServer, numGCPrev, "num_gc"))

	// 4. GC Pause Total (convert from nanoseconds to milliseconds)
	seg = traceway.StartSegment(c, "loading gc pause")
	gcPausePerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameGCPauseTotal, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading gcPausePerServer: %w", err))
		return
	}
	// Convert nanoseconds to milliseconds for each server's data
	for serverName, points := range gcPausePerServer {
		for i := range points {
			gcPausePerServer[serverName][i].Value = points[i].Value / 1_000_000
		}
	}
	gcPausePrevRaw, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameGCPauseTotal, prevStart, prevEnd)
	seg.End()
	gcPausePrev := gcPausePrevRaw / 1_000_000
	metrics = append(metrics, buildMetricWithServers("gc_pause", "GC Pause", "ms", gcPausePerServer, gcPausePrev, "gc_pause"))

	c.JSON(http.StatusOK, ApplicationMetricsResponse{
		Metrics:          metrics,
		AvailableServers: availableServers,
		LastUpdated:      now,
	})
}

// GetStatsMetrics returns request/exception stats (Requests, Exceptions, Avg Response Time, Error Rate)
// These are NOT per-server metrics
func (m metricsController) GetStatsMetrics(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	now := time.Now()
	start, end := parseTimeRange(c, now)

	// Calculate previous period for comparison
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	// Calculate aggregation interval based on time range
	intervalMinutes := calculateIntervalMinutes(duration)

	metrics := make([]models.DashboardMetric, 0, 4)

	// 1. Requests count
	seg := traceway.StartSegment(c, "loading requests trend")
	requestsTrend, err := repositories.EndpointRepository.CountByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading requestsTrend: %w", err))
		return
	}
	requestsCurrent, _ := repositories.EndpointRepository.CountBetween(c, projectId, start, end)
	requestsPrev, _ := repositories.EndpointRepository.CountBetween(c, projectId, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetric("requests", "Requests", float64(requestsCurrent), "count", requestsTrend, float64(requestsPrev), "requests"))

	// 2. Exceptions count
	seg = traceway.StartSegment(c, "loading exceptions trend")
	exceptionsTrend, err := repositories.ExceptionStackTraceRepository.CountByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptionsTrend: %w", err))
		return
	}
	exceptionsCurrent, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, start, end)
	exceptionsPrev, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetric("exceptions", "Exceptions", float64(exceptionsCurrent), "count", exceptionsTrend, float64(exceptionsPrev), "exceptions"))

	// 3. Average Response Time
	seg = traceway.StartSegment(c, "loading avg response time")
	avgDurationTrend, err := repositories.EndpointRepository.AvgDurationByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading avgDurationTrend: %w", err))
		return
	}
	avgDurationCurrent := getLastValue(avgDurationTrend)
	avgDurationPrevTrend, _ := repositories.EndpointRepository.AvgDurationByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	avgDurationPrev := getAverageValue(avgDurationPrevTrend)
	seg.End()
	metrics = append(metrics, buildMetric("avg_response_time", "Avg Response Time", avgDurationCurrent, "ms", avgDurationTrend, avgDurationPrev, "response_time"))

	// 4. Error Rate
	seg = traceway.StartSegment(c, "loading error rate")
	errorRateTrend, err := repositories.EndpointRepository.ErrorRateByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading errorRateTrend: %w", err))
		return
	}
	errorRateCurrent := getLastValue(errorRateTrend)
	errorRatePrevTrend, _ := repositories.EndpointRepository.ErrorRateByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	errorRatePrev := getAverageValue(errorRatePrevTrend)
	seg.End()
	metrics = append(metrics, buildMetric("error_rate", "Error Rate", errorRateCurrent, "%", errorRateTrend, errorRatePrev, "error_rate"))

	c.JSON(http.StatusOK, StatsMetricsResponse{
		Metrics:     metrics,
		LastUpdated: now,
	})
}

// GetServerMetrics returns server resource metrics (CPU Usage, Memory Usage, Total Memory)
// Always returns ALL servers' data - ignores server selector
func (m metricsController) GetServerMetrics(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	now := time.Now()
	start, end := parseTimeRange(c, now)

	// Calculate previous period for comparison
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	// Calculate aggregation interval based on time range
	intervalMinutes := calculateIntervalMinutes(duration)

	// Get available servers in the time range
	availableServers, err := repositories.MetricRecordRepository.GetDistinctServers(c, projectId, start, end)
	if err != nil {
		availableServers = []string{}
	}

	// Always pass empty slice to get ALL servers
	var emptyServers []string

	metrics := make([]models.DashboardMetric, 0, 3)

	// 1. CPU Usage
	seg := traceway.StartSegment(c, "loading cpu usage")
	cpuPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameCpuUsage, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading cpuPerServer: %w", err))
		return
	}
	cpuPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("cpu_usage", "CPU Usage", "%", cpuPerServer, cpuPrev, "cpu"))

	// 2. Memory Usage (MB)
	seg = traceway.StartSegment(c, "loading memory usage")
	memPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameMemoryUsage, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memPerServer: %w", err))
		return
	}
	memPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("memory_usage", "Memory Usage", "MB", memPerServer, memPrev, "memory"))

	// 3. Total System Memory (MB)
	seg = traceway.StartSegment(c, "loading total memory")
	memTotalPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameMemoryTotal, start, end, intervalMinutes, emptyServers)
	if err != nil {
		seg.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memTotalPerServer: %w", err))
		return
	}
	memTotalPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryTotal, prevStart, prevEnd)
	seg.End()
	metrics = append(metrics, buildMetricWithServers("memory_total", "Total Memory", "MB", memTotalPerServer, memTotalPrev, "memory_total"))

	c.JSON(http.StatusOK, ServerMetricsResponse{
		Metrics:          metrics,
		AvailableServers: availableServers,
		LastUpdated:      now,
	})
}

func parseTimeRange(c *gin.Context, now time.Time) (start, end time.Time) {
	if fromDateStr := c.Query("fromDate"); fromDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			start = parsed
		} else {
			start = now.Add(-24 * time.Hour)
		}
	} else {
		start = now.Add(-24 * time.Hour)
	}

	if toDateStr := c.Query("toDate"); toDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			end = parsed
		} else {
			end = now
		}
	} else {
		end = now
	}

	return start, end
}

var MetricsController = metricsController{}
