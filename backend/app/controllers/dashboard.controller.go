package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	traceway "go.tracewayapp.com"
)

type dashboardController struct{}

type DashboardOverviewResponse struct {
	RecentIssues   []models.ExceptionGroup `json:"recentIssues"`
	WorstEndpoints []models.EndpointStats  `json:"worstEndpoints"`
	HasData        bool                    `json:"hasData"`
}

func (d dashboardController) GetDashboardOverview(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	// Get last 10 issues in the last 24 hours (only exceptions, not messages)
	recentIssues, _, err := repositories.ExceptionStackTraceRepository.FindGrouped(c, projectId, start, now, 1, 10, "last_seen", "", "issues", false)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading recent issues: %w", err))
		return
	}

	// Get 10 worst performing endpoints
	worstEndpoints, err := repositories.EndpointRepository.FindWorstEndpoints(c, projectId, start, now, 10)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading worst endpoints: %w", err))
		return
	}

	hasData := true
	if len(worstEndpoints) == 0 && len(recentIssues) == 0 {
		// Check if project has received ANY data (all time, not just 24h)
		var epoch time.Time // zero time = beginning of time
		endpointCount, _ := repositories.EndpointRepository.CountBetween(c, projectId, epoch, now)
		exceptionCount, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, epoch, now)
		hasData = endpointCount > 0 || exceptionCount > 0
	}

	c.JSON(http.StatusOK, DashboardOverviewResponse{
		RecentIssues:   recentIssues,
		WorstEndpoints: worstEndpoints,
		HasData:        hasData,
	})
}

func (d dashboardController) GetDashboard(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	serversParam := c.Query("servers")

	// Parse selected servers
	var selectedServers []string
	if serversParam != "" {
		selectedServers = strings.Split(serversParam, ",")
	}

	now := time.Now()
	var start, end time.Time

	// Parse fromDate parameter
	if fromDateStr := c.Query("fromDate"); fromDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromDateStr); err == nil {
			start = parsed
		} else {
			start = now.Add(-24 * time.Hour)
		}
	} else {
		start = now.Add(-24 * time.Hour)
	}

	// Parse toDate parameter
	if toDateStr := c.Query("toDate"); toDateStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toDateStr); err == nil {
			end = parsed
		} else {
			end = now
		}
	} else {
		end = now
	}

	// Calculate previous period for comparison (same duration before start)
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

	metrics := make([]models.DashboardMetric, 0, 11)

	// 1. Requests count
	requestsTrend, err := repositories.EndpointRepository.CountByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading requestsTrend: %w", err))
		return
	}
	requestsCurrent, _ := repositories.EndpointRepository.CountBetween(c, projectId, start, end)
	requestsPrev, _ := repositories.EndpointRepository.CountBetween(c, projectId, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("requests", "Requests", float64(requestsCurrent), "count", requestsTrend, float64(requestsPrev), "requests"))

	// 2. Exceptions count
	exceptionsTrend, err := repositories.ExceptionStackTraceRepository.CountByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptionsTrend: %w", err))
		return
	}
	exceptionsCurrent, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, start, end)
	exceptionsPrev, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("exceptions", "Exceptions", float64(exceptionsCurrent), "count", exceptionsTrend, float64(exceptionsPrev), "exceptions"))

	// 3. Average Response Time
	avgDurationTrend, err := repositories.EndpointRepository.AvgDurationByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading avgDurationTrend: %w", err))
		return
	}
	avgDurationCurrent := getLastValue(avgDurationTrend)
	avgDurationPrevTrend, _ := repositories.EndpointRepository.AvgDurationByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	avgDurationPrev := getAverageValue(avgDurationPrevTrend)
	metrics = append(metrics, buildMetric("avg_response_time", "Avg Response Time", avgDurationCurrent, "ms", avgDurationTrend, avgDurationPrev, "response_time"))

	// 4. Error Rate
	errorRateTrend, err := repositories.EndpointRepository.ErrorRateByInterval(c, projectId, start, end, intervalMinutes)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading errorRateTrend: %w", err))
		return
	}
	errorRateCurrent := getLastValue(errorRateTrend)
	errorRatePrevTrend, _ := repositories.EndpointRepository.ErrorRateByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	errorRatePrev := getAverageValue(errorRatePrevTrend)
	metrics = append(metrics, buildMetric("error_rate", "Error Rate", errorRateCurrent, "%", errorRateTrend, errorRatePrev, "error_rate"))

	// 5. CPU Usage
	cpuPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameCpuUsage, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading cpuPerServer: %w", err))
		return
	}
	cpuPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("cpu_usage", "CPU Usage", "%", cpuPerServer, cpuPrev, "cpu"))

	// 6. Memory Usage (MB)
	memPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameMemoryUsage, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memPerServer: %w", err))
		return
	}
	memPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("memory_usage", "Memory Usage", "MB", memPerServer, memPrev, "memory"))

	// 7. Total System Memory (MB)
	memTotalPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameMemoryTotal, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memTotalPerServer: %w", err))
		return
	}
	memTotalPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryTotal, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("memory_total", "Total Memory", "MB", memTotalPerServer, memTotalPrev, "memory_total"))

	// 8. Go Routines
	goRoutinesPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameGoRoutines, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading goRoutinesPerServer: %w", err))
		return
	}
	goRoutinesPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameGoRoutines, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("go_routines", "Go Routines", "", goRoutinesPerServer, goRoutinesPrev, "go_routines"))

	// 9. Heap Objects
	heapObjectsPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameHeapObjects, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading heapObjectsPerServer: %w", err))
		return
	}
	heapObjectsPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameHeapObjects, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("heap_objects", "Heap Objects", "", heapObjectsPerServer, heapObjectsPrev, "heap_objects"))

	// 10. Num GC
	numGCPerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameNumGC, start, end, intervalMinutes, selectedServers)
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading numGCPerServer: %w", err))
		return
	}
	numGCPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameNumGC, prevStart, prevEnd)
	metrics = append(metrics, buildMetricWithServers("num_gc", "GC Cycles", "", numGCPerServer, numGCPrev, "num_gc"))

	// 11. GC Pause Total (convert from nanoseconds to milliseconds)
	gcPausePerServer, err := repositories.MetricRecordRepository.GetAverageByIntervalPerServer(c, projectId, models.MetricNameGCPauseTotal, start, end, intervalMinutes, selectedServers)
	if err != nil {
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
	gcPausePrev := gcPausePrevRaw / 1_000_000
	metrics = append(metrics, buildMetricWithServers("gc_pause", "GC Pause", "ms", gcPausePerServer, gcPausePrev, "gc_pause"))

	c.JSON(http.StatusOK, models.DashboardResponse{
		Metrics:          metrics,
		AvailableServers: availableServers,
		LastUpdated:      now,
	})
}

func buildMetric(id, name string, current float64, unit string, trend []models.TimeSeriesPoint, prev float64, metricType string) models.DashboardMetric {
	// Append unit to name if meaningful (not empty and not "count")
	displayName := name
	if unit != "" && unit != "count" {
		displayName = name + " (" + unit + ")"
	}

	// Convert TimeSeriesPoint to DashboardTrendPoint
	trendPoints := make([]models.DashboardTrendPoint, len(trend))
	for i, p := range trend {
		trendPoints[i] = models.DashboardTrendPoint{
			Timestamp: p.Timestamp,
			Value:     p.Value,
		}
	}

	// Calculate percentage change
	var change24h float64
	if prev > 0 {
		change24h = ((current - prev) / prev) * 100
	}

	// Determine status based on metric type
	status := calculateStatus(current, metricType)

	return models.DashboardMetric{
		ID:        id,
		Name:      displayName,
		Value:     current,
		Unit:      unit,
		Trend:     trendPoints,
		Change24h: change24h,
		Status:    status,
	}
}

func buildMetricWithServers(id, name, unit string, serverData map[string][]models.TimeSeriesPoint, prev float64, metricType string) models.DashboardMetric {
	// Append unit to name if meaningful (not empty and not "count")
	displayName := name
	if unit != "" && unit != "count" {
		displayName = name + " (" + unit + ")"
	}

	servers := make([]models.ServerMetricTrend, 0, len(serverData))
	var aggregateValue float64
	var aggregateTrend []models.DashboardTrendPoint

	// Build server-level data
	serverNames := make([]string, 0, len(serverData))
	for serverName := range serverData {
		serverNames = append(serverNames, serverName)
	}
	sort.Strings(serverNames)

	// Merge all timestamps for aggregate trend
	timestampValues := make(map[time.Time][]float64)

	for _, serverName := range serverNames {
		trend := serverData[serverName]
		trendPoints := make([]models.DashboardTrendPoint, len(trend))
		var lastValue float64

		for i, p := range trend {
			trendPoints[i] = models.DashboardTrendPoint{
				Timestamp: p.Timestamp,
				Value:     p.Value,
			}
			lastValue = p.Value
			timestampValues[p.Timestamp] = append(timestampValues[p.Timestamp], p.Value)
		}

		servers = append(servers, models.ServerMetricTrend{
			ServerName: serverName,
			Value:      lastValue,
			Trend:      trendPoints,
		})
	}

	// Calculate aggregate value (average of last values across servers)
	if len(servers) > 0 {
		for _, s := range servers {
			aggregateValue += s.Value
		}
		aggregateValue /= float64(len(servers))
	}

	// Build aggregate trend (average at each timestamp)
	timestamps := make([]time.Time, 0, len(timestampValues))
	for ts := range timestampValues {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	for _, ts := range timestamps {
		values := timestampValues[ts]
		var sum float64
		for _, v := range values {
			sum += v
		}
		aggregateTrend = append(aggregateTrend, models.DashboardTrendPoint{
			Timestamp: ts,
			Value:     sum / float64(len(values)),
		})
	}

	// Calculate percentage change
	var change24h float64
	if prev > 0 {
		change24h = ((aggregateValue - prev) / prev) * 100
	}

	// Determine status based on metric type
	status := calculateStatus(aggregateValue, metricType)

	return models.DashboardMetric{
		ID:        id,
		Name:      displayName,
		Value:     aggregateValue,
		Unit:      unit,
		Trend:     aggregateTrend,
		Servers:   servers,
		Change24h: change24h,
		Status:    status,
	}
}

func calculateStatus(value float64, metricType string) string {
	switch metricType {
	case "requests":
		// Lower is worse for requests (less traffic may indicate issues)
		if value < 10 {
			return "critical"
		} else if value < 100 {
			return "warning"
		}
		return "healthy"
	case "exceptions":
		// Higher is worse for exceptions
		if value > 50 {
			return "critical"
		} else if value > 10 {
			return "warning"
		}
		return "healthy"
	case "response_time":
		// Higher is worse for response time
		if value > 500 {
			return "critical"
		} else if value > 200 {
			return "warning"
		}
		return "healthy"
	case "error_rate":
		// Higher is worse for error rate
		if value > 5 {
			return "critical"
		} else if value > 2 {
			return "warning"
		}
		return "healthy"
	case "cpu":
		// Higher is worse for CPU
		if value > 90 {
			return "critical"
		} else if value > 70 {
			return "warning"
		}
		return "healthy"
	case "memory":
		// Higher is worse for memory (MB)
		if value > 900 {
			return "critical"
		} else if value > 700 {
			return "warning"
		}
		return "healthy"
	case "memory_total":
		// Total memory is informational - always healthy
		return "healthy"
	case "go_routines":
		// Higher may indicate goroutine leaks
		if value > 10000 {
			return "critical"
		} else if value > 5000 {
			return "warning"
		}
		return "healthy"
	case "heap_objects":
		// Higher may indicate memory pressure
		if value > 1000000 {
			return "critical"
		} else if value > 500000 {
			return "warning"
		}
		return "healthy"
	case "num_gc", "gc_pause":
		// These are informational, always healthy
		return "healthy"
	}
	return "healthy"
}

func getLastValue(points []models.TimeSeriesPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	return points[len(points)-1].Value
}

func getAverageValue(points []models.TimeSeriesPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	var sum float64
	for _, p := range points {
		sum += p.Value
	}
	return sum / float64(len(points))
}

// calculateIntervalMinutes determines the aggregation bucket size based on the time range duration
func calculateIntervalMinutes(duration time.Duration) int {
	hours := duration.Hours()
	switch {
	case hours < 2:
		return 1 // 1-minute buckets
	case hours < 12:
		return 5 // 5-minute buckets
	case hours < 48:
		return 15 // 15-minute buckets
	case hours < 168: // 7 days
		return 60 // 1-hour buckets
	default:
		return 240 // 4-hour buckets
	}
}

var DashboardController = dashboardController{}
