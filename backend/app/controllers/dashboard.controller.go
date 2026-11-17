package controllers

import (
	"context"
	"fmt"

	"github.com/tracewayapp/traceway/backend/v2/app/cache"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry"
	"github.com/tracewayapp/traceway/backend/v2/app/traceway"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	traceway.SetAttribute(c, "project_id", projectId.String())

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	// Get last 10 issues in the last 24 hours (only exceptions, not messages)
	spanCtx, span := traceway.StartSpan(c, "loading recent issues")
	recentIssues, _, err := telemetry.ExceptionStackTraceRepository.FindGrouped(spanCtx, projectId, start, now, 1, 10, "last_seen", "", "issues", false)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading recent issues: %w", err))
		return
	}

	// Get 10 worst performing endpoints
	spanCtx, span = traceway.StartSpan(c, "loading worst endpoints")
	worstEndpoints, err := telemetry.EndpointRepository.FindWorstEndpoints(spanCtx, projectId, start, now, 10)
	span.End()
	if err != nil {
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading worst endpoints: %w", err))
		return
	}

	hasData := true
	if len(worstEndpoints) == 0 && len(recentIssues) == 0 {
		// A count past the deadline finishes after the request, so it must
		// not hold the gin context, which gin reuses for the next request.
		spanCtx, span = traceway.StartSpan(context.WithoutCancel(c.Request.Context()), "checking project data")
		hasData = projectHasData(spanCtx, projectId, now)
		span.End()
	}

	c.JSON(http.StatusOK, DashboardOverviewResponse{
		RecentIssues:   recentIssues,
		WorstEndpoints: worstEndpoints,
		HasData:        hasData,
	})
}

var (
	hasDataTimeout   = 10 * time.Second
	countProjectData = countEndpointsAndExceptions

	// Only a counted yes is remembered, for the life of the process: the
	// answer turns back into no only when retention removes a project's last
	// row, and showing that project an empty dashboard until the next restart
	// costs less than repeating the count on every visit.
	projectsWithData sync.Map
)

// A false answer swaps the dashboard for the setup guide, so a count that
// fails or times out must not be read as "this project never sent data".
// The SQLite and DuckDB reads cannot be cancelled (lit has no context), so
// the deadline is kept here: a count past it finishes in the background and
// its answer is dropped.
func projectHasData(ctx context.Context, projectId uuid.UUID, now time.Time) bool {
	if _, known := projectsWithData.Load(projectId); known {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, hasDataTimeout)
	defer cancel()

	type answer struct {
		hasData bool
		err     error
	}
	result := make(chan answer, 1)
	go func() {
		a := answer{hasData: true}
		defer func() { result <- a }()
		defer traceway.Recover()
		a.hasData, a.err = countProjectData(ctx, projectId, now)
		if a.err == nil && a.hasData {
			projectsWithData.Store(projectId, struct{}{})
		}
	}()

	select {
	case a := <-result:
		if a.err != nil {
			traceway.CaptureExceptionContext(ctx, fmt.Errorf("dashboard overview: checking whether project %s has data: %w", projectId, a.err))
			return true
		}
		return a.hasData
	case <-ctx.Done():
		traceway.CaptureExceptionContext(ctx, fmt.Errorf("dashboard overview: checking whether project %s has data: %w", projectId, ctx.Err()))
		return true
	}
}

// Newest first, and never before the project existed: the count over all time
// is the one read of this page with no time bound, and on ClickHouse it has to
// open every daily partition of the table even for a project with no rows in
// it (10s and a timeout on a project with zero endpoints, while the same table
// answered a 24h read in the same request). A project that is quiet today
// almost always has rows in the last 30 days, and a new project has nothing
// older than itself, so the unbounded read is left to old projects that have
// been silent for a month.
func hasDataWindows(projectId uuid.UUID, now time.Time) []hasDataWindow {
	var oldest time.Time
	if project := cache.ProjectCache.GetById(projectId); project != nil {
		oldest = project.CreatedAt.Add(-hasDataClockSlack)
	}
	recentStart := now.Add(-hasDataRecentWindow)
	if !oldest.Before(recentStart) {
		return []hasDataWindow{{"recent", oldest, now}}
	}
	return []hasDataWindow{{"recent", recentStart, now}, {"older", oldest, recentStart}}
}

type hasDataWindow struct {
	name       string
	start, end time.Time
}

const (
	hasDataRecentWindow = 30 * 24 * time.Hour
	hasDataClockSlack   = 24 * time.Hour
)

func countEndpointsAndExceptions(ctx context.Context, projectId uuid.UUID, now time.Time) (bool, error) {
	for _, window := range hasDataWindows(projectId, now) {
		countCtx, countSpan := traceway.StartSpan(ctx, "dashboard.count_endpoints")
		traceway.SetAttribute(countCtx, "dashboard.window", window.name)
		endpointCount, err := telemetry.EndpointRepository.CountBetween(countCtx, projectId, window.start, window.end)
		if err == nil {
			traceway.SetAttribute(countCtx, "dashboard.record_count", fmt.Sprint(endpointCount))
		}
		countSpan.End()
		if err != nil {
			return true, fmt.Errorf("counting endpoints (%s): %w", window.name, err)
		}
		if endpointCount > 0 {
			return true, nil
		}
		countCtx, countSpan = traceway.StartSpan(ctx, "dashboard.count_exceptions")
		traceway.SetAttribute(countCtx, "dashboard.window", window.name)
		exceptionCount, err := telemetry.ExceptionStackTraceRepository.CountBetween(countCtx, projectId, window.start, window.end)
		if err == nil {
			traceway.SetAttribute(countCtx, "dashboard.record_count", fmt.Sprint(exceptionCount))
		}
		countSpan.End()
		if err != nil {
			return true, fmt.Errorf("counting exceptions (%s): %w", window.name, err)
		}
		if exceptionCount > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (d dashboardController) GetDashboard(c *gin.Context) {
	projectId, err := uuid.Parse(c.Query("projectId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
		return
	}
	serversParam := c.Query("servers")

	var selectedServers []string
	if serversParam != "" {
		selectedServers = strings.Split(serversParam, ",")
	}

	now := time.Now()
	var start, end time.Time

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

	// Calculate previous period for comparison (same duration before start)
	duration := end.Sub(start)
	prevStart := start.Add(-duration)
	prevEnd := start

	// Calculate aggregation interval based on time range
	intervalMinutes := calculateIntervalMinutes(duration)

	// Get available servers in the time range
	availableServers, err := telemetry.MetricPointRepository.GetDistinctServers(c, projectId, start, end)
	if err != nil {
		availableServers = []string{}
	}

	metrics := make([]models.DashboardMetric, 0, 11)

	// 1. Requests count
	spanCtx, span := traceway.StartSpan(c, "loading requests trend")
	requestsTrend, err := telemetry.EndpointRepository.CountByInterval(spanCtx, projectId, start, end, intervalMinutes)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading requestsTrend: %w", err))
		return
	}
	requestsCurrent, _ := telemetry.EndpointRepository.CountBetween(c, projectId, start, end)
	requestsPrev, _ := telemetry.EndpointRepository.CountBetween(c, projectId, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetric("requests", "Requests", float64(requestsCurrent), "count", requestsTrend, float64(requestsPrev), "requests"))

	// 2. Exceptions count
	spanCtx, span = traceway.StartSpan(c, "loading exceptions trend")
	exceptionsTrend, err := telemetry.ExceptionStackTraceRepository.CountByInterval(spanCtx, projectId, start, end, intervalMinutes)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading exceptionsTrend: %w", err))
		return
	}
	exceptionsCurrent, _ := telemetry.ExceptionStackTraceRepository.CountBetween(c, projectId, start, end)
	exceptionsPrev, _ := telemetry.ExceptionStackTraceRepository.CountBetween(c, projectId, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetric("exceptions", "Exceptions", float64(exceptionsCurrent), "count", exceptionsTrend, float64(exceptionsPrev), "exceptions"))

	// 3. Average Response Time
	spanCtx, span = traceway.StartSpan(c, "loading avg response time")
	avgDurationTrend, err := telemetry.EndpointRepository.AvgDurationByInterval(spanCtx, projectId, start, end, intervalMinutes)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading avgDurationTrend: %w", err))
		return
	}
	avgDurationCurrent := getLastValue(avgDurationTrend)
	avgDurationPrevTrend, _ := telemetry.EndpointRepository.AvgDurationByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	avgDurationPrev := getAverageValue(avgDurationPrevTrend)
	span.End()
	metrics = append(metrics, buildMetric("avg_response_time", "Avg Response Time", avgDurationCurrent, "ms", avgDurationTrend, avgDurationPrev, "response_time"))

	// 4. Error Rate
	spanCtx, span = traceway.StartSpan(c, "loading error rate")
	errorRateTrend, err := telemetry.EndpointRepository.ErrorRateByInterval(spanCtx, projectId, start, end, intervalMinutes)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading errorRateTrend: %w", err))
		return
	}
	errorRateCurrent := getLastValue(errorRateTrend)
	errorRatePrevTrend, _ := telemetry.EndpointRepository.ErrorRateByInterval(c, projectId, prevStart, prevEnd, intervalMinutes)
	errorRatePrev := getAverageValue(errorRatePrevTrend)
	span.End()
	metrics = append(metrics, buildMetric("error_rate", "Error Rate", errorRateCurrent, "%", errorRateTrend, errorRatePrev, "error_rate"))

	// 5. CPU Usage
	spanCtx, span = traceway.StartSpan(c, "loading cpu usage")
	cpuPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameCpuUsage, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading cpuPerServer: %w", err))
		return
	}
	cpuPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("cpu_usage", "CPU Usage", "%", cpuPerServer, cpuPrev, "cpu"))

	// 6. Memory Usage (MB)
	spanCtx, span = traceway.StartSpan(c, "loading memory usage")
	memPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameMemoryUsage, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memPerServer: %w", err))
		return
	}
	memPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("memory_usage", "Memory Usage", "MB", memPerServer, memPrev, "memory"))

	// 7. Total System Memory (MB)
	spanCtx, span = traceway.StartSpan(c, "loading total memory")
	memTotalPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameMemoryTotal, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading memTotalPerServer: %w", err))
		return
	}
	memTotalPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryTotal, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("memory_total", "Total Memory", "MB", memTotalPerServer, memTotalPrev, "memory_total"))

	// 8. Go Routines
	spanCtx, span = traceway.StartSpan(c, "loading go routines")
	goRoutinesPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameGoRoutines, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading goRoutinesPerServer: %w", err))
		return
	}
	goRoutinesPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameGoRoutines, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("go_routines", "Go Routines", "", goRoutinesPerServer, goRoutinesPrev, "go_routines"))

	// 9. Heap Objects
	spanCtx, span = traceway.StartSpan(c, "loading heap objects")
	heapObjectsPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameHeapObjects, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading heapObjectsPerServer: %w", err))
		return
	}
	heapObjectsPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameHeapObjects, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("heap_objects", "Heap Objects", "", heapObjectsPerServer, heapObjectsPrev, "heap_objects"))

	// 10. Num GC
	spanCtx, span = traceway.StartSpan(c, "loading gc cycles")
	numGCPerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameNumGC, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading numGCPerServer: %w", err))
		return
	}
	numGCPrev, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameNumGC, prevStart, prevEnd)
	span.End()
	metrics = append(metrics, buildMetricWithServers("num_gc", "GC Cycles", "", numGCPerServer, numGCPrev, "num_gc"))

	// 11. GC Pause Total (convert from nanoseconds to milliseconds)
	spanCtx, span = traceway.StartSpan(c, "loading gc pause")
	gcPausePerServer, err := telemetry.MetricPointRepository.GetAverageByIntervalPerServer(spanCtx, projectId, models.MetricNameGCPauseTotal, start, end, intervalMinutes, selectedServers)
	if err != nil {
		span.End()
		c.AbortWithError(500, traceway.NewStackTraceErrorf("error loading gcPausePerServer: %w", err))
		return
	}
	// Convert nanoseconds to milliseconds for each server's data
	for serverName, points := range gcPausePerServer {
		for i := range points {
			gcPausePerServer[serverName][i].Value = points[i].Value / 1_000_000
		}
	}
	gcPausePrevRaw, _ := telemetry.MetricPointRepository.GetAverageBetween(c, projectId, models.MetricNameGCPauseTotal, prevStart, prevEnd)
	span.End()
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
