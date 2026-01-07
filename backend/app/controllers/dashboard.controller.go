package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type dashboardController struct{}

type DashboardOverviewResponse struct {
	RecentIssues    []models.ExceptionGroup `json:"recentIssues"`
	WorstEndpoints  []models.EndpointStats  `json:"worstEndpoints"`
}

func (d dashboardController) GetDashboardOverview(c *gin.Context) {
	projectId := c.Query("projectId")

	now := time.Now()
	start := now.Add(-24 * time.Hour)

	// Get last 10 issues in the last 24 hours
	recentIssues, _, err := repositories.ExceptionStackTraceRepository.FindGrouped(c, projectId, start, now, 1, 10, "last_seen", "")
	if err != nil {
		panic(err)
	}

	// Get 10 worst performing endpoints
	worstEndpoints, err := repositories.TransactionRepository.FindWorstEndpoints(c, projectId, start, now, 10)
	if err != nil {
		panic(err)
	}

	c.JSON(http.StatusOK, DashboardOverviewResponse{
		RecentIssues:   recentIssues,
		WorstEndpoints: worstEndpoints,
	})
}

func (d dashboardController) GetDashboard(c *gin.Context) {
	projectId := c.Query("projectId")

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	prevStart := now.Add(-48 * time.Hour)
	prevEnd := start

	metrics := make([]models.DashboardMetric, 0, 11)

	// 1. Requests count
	requestsTrend, err := repositories.TransactionRepository.CountByHour(c, projectId, start, now)
	if err != nil {
		panic(err)
	}
	requestsCurrent, _ := repositories.TransactionRepository.CountBetween(c, projectId, start, now)
	requestsPrev, _ := repositories.TransactionRepository.CountBetween(c, projectId, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("requests", "Requests", float64(requestsCurrent), "count", requestsTrend, float64(requestsPrev), "requests"))

	// 2. Exceptions count
	exceptionsTrend, err := repositories.ExceptionStackTraceRepository.CountByHour(c, projectId, start, now)
	if err != nil {
		panic(err)
	}
	exceptionsCurrent, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, start, now)
	exceptionsPrev, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("exceptions", "Exceptions", float64(exceptionsCurrent), "count", exceptionsTrend, float64(exceptionsPrev), "exceptions"))

	// 3. Average Response Time
	avgDurationTrend, err := repositories.TransactionRepository.AvgDurationByHour(c, projectId, start, now)
	if err != nil {
		panic(err)
	}
	avgDurationCurrent := getLastValue(avgDurationTrend)
	avgDurationPrevTrend, _ := repositories.TransactionRepository.AvgDurationByHour(c, projectId, prevStart, prevEnd)
	avgDurationPrev := getAverageValue(avgDurationPrevTrend)
	metrics = append(metrics, buildMetric("avg_response_time", "Avg Response Time", avgDurationCurrent, "ms", avgDurationTrend, avgDurationPrev, "response_time"))

	// 4. Error Rate
	errorRateTrend, err := repositories.TransactionRepository.ErrorRateByHour(c, projectId, start, now)
	if err != nil {
		panic(err)
	}
	errorRateCurrent := getLastValue(errorRateTrend)
	errorRatePrevTrend, _ := repositories.TransactionRepository.ErrorRateByHour(c, projectId, prevStart, prevEnd)
	errorRatePrev := getAverageValue(errorRatePrevTrend)
	metrics = append(metrics, buildMetric("error_rate", "Error Rate", errorRateCurrent, "%", errorRateTrend, errorRatePrev, "error_rate"))

	// 5. CPU Usage
	cpuTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameCpuUsage, start, now)
	if err != nil {
		panic(err)
	}
	cpuCurrent := getLastValue(cpuTrend)
	cpuPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("cpu_usage", "CPU Usage", cpuCurrent, "%", cpuTrend, cpuPrev, "cpu"))

	// 6. Memory Usage (MB)
	memTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameMemoryUsage, start, now)
	if err != nil {
		panic(err)
	}
	memCurrent := getLastValue(memTrend)
	memPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("memory_usage", "Memory Usage", memCurrent, "MB", memTrend, memPrev, "memory"))

	// 7. Memory Usage Percentage
	memPcntTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameMemoryUsagePcnt, start, now)
	if err != nil {
		panic(err)
	}
	memPcntCurrent := getLastValue(memPcntTrend)
	memPcntPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsagePcnt, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("memory_usage_pcnt", "Memory %", memPcntCurrent, "%", memPcntTrend, memPcntPrev, "memory_pcnt"))

	// 8. Go Routines
	goRoutinesTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameGoRoutines, start, now)
	if err != nil {
		panic(err)
	}
	goRoutinesCurrent := getLastValue(goRoutinesTrend)
	goRoutinesPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameGoRoutines, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("go_routines", "Go Routines", goRoutinesCurrent, "", goRoutinesTrend, goRoutinesPrev, "go_routines"))

	// 9. Heap Objects
	heapObjectsTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameHeapObjects, start, now)
	if err != nil {
		panic(err)
	}
	heapObjectsCurrent := getLastValue(heapObjectsTrend)
	heapObjectsPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameHeapObjects, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("heap_objects", "Heap Objects", heapObjectsCurrent, "", heapObjectsTrend, heapObjectsPrev, "heap_objects"))

	// 10. Num GC
	numGCTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameNumGC, start, now)
	if err != nil {
		panic(err)
	}
	numGCCurrent := getLastValue(numGCTrend)
	numGCPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameNumGC, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("num_gc", "GC Cycles", numGCCurrent, "", numGCTrend, numGCPrev, "num_gc"))

	// 11. GC Pause Total (convert from nanoseconds to milliseconds)
	gcPauseTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, projectId, models.MetricNameGCPauseTotal, start, now)
	if err != nil {
		panic(err)
	}
	// Convert nanoseconds to milliseconds for display
	for i := range gcPauseTrend {
		gcPauseTrend[i].Value = gcPauseTrend[i].Value / 1_000_000
	}
	gcPauseCurrent := getLastValue(gcPauseTrend)
	gcPausePrevRaw, _ := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameGCPauseTotal, prevStart, prevEnd)
	gcPausePrev := gcPausePrevRaw / 1_000_000
	metrics = append(metrics, buildMetric("gc_pause", "GC Pause", gcPauseCurrent, "ms", gcPauseTrend, gcPausePrev, "gc_pause"))

	c.JSON(http.StatusOK, models.DashboardResponse{
		Metrics:     metrics,
		LastUpdated: now,
	})
}

func buildMetric(id, name string, current float64, unit string, trend []models.TimeSeriesPoint, prev float64, metricType string) models.DashboardMetric {
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
		Name:      name,
		Value:     current,
		Unit:      unit,
		Trend:     trendPoints,
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
	case "memory_pcnt":
		// Higher is worse for memory percentage
		if value > 90 {
			return "critical"
		} else if value > 70 {
			return "warning"
		}
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

var DashboardController = dashboardController{}
