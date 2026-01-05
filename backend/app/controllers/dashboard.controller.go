package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type dashboardController struct{}

func (d dashboardController) GetDashboard(c *gin.Context) {
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	prevStart := now.Add(-48 * time.Hour)
	prevEnd := start

	metrics := make([]models.DashboardMetric, 0, 6)

	// 1. Requests count
	requestsTrend, err := repositories.TransactionRepository.CountByHour(c, start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	requestsCurrent, _ := repositories.TransactionRepository.CountBetween(c, start, now)
	requestsPrev, _ := repositories.TransactionRepository.CountBetween(c, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("requests", "Requests", float64(requestsCurrent), "count", requestsTrend, float64(requestsPrev), "requests"))

	// 2. Exceptions count
	exceptionsTrend, err := repositories.ExceptionStackTraceRepository.CountByHour(c, start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	exceptionsCurrent, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, start, now)
	exceptionsPrev, _ := repositories.ExceptionStackTraceRepository.CountBetween(c, prevStart, prevEnd)
	metrics = append(metrics, buildMetric("exceptions", "Exceptions", float64(exceptionsCurrent), "count", exceptionsTrend, float64(exceptionsPrev), "exceptions"))

	// 3. Average Response Time
	avgDurationTrend, err := repositories.TransactionRepository.AvgDurationByHour(c, start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	avgDurationCurrent := getLastValue(avgDurationTrend)
	avgDurationPrevTrend, _ := repositories.TransactionRepository.AvgDurationByHour(c, prevStart, prevEnd)
	avgDurationPrev := getAverageValue(avgDurationPrevTrend)
	metrics = append(metrics, buildMetric("avg_response_time", "Avg Response Time", avgDurationCurrent, "ms", avgDurationTrend, avgDurationPrev, "response_time"))

	// 4. Error Rate
	errorRateTrend, err := repositories.TransactionRepository.ErrorRateByHour(c, start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	errorRateCurrent := getLastValue(errorRateTrend)
	errorRatePrevTrend, _ := repositories.TransactionRepository.ErrorRateByHour(c, prevStart, prevEnd)
	errorRatePrev := getAverageValue(errorRatePrevTrend)
	metrics = append(metrics, buildMetric("error_rate", "Error Rate", errorRateCurrent, "%", errorRateTrend, errorRatePrev, "error_rate"))

	// 5. CPU Usage
	cpuTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, "cpu", start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	cpuCurrent := getLastValue(cpuTrend)
	cpuPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, "cpu", prevStart, prevEnd)
	metrics = append(metrics, buildMetric("cpu_usage", "CPU Usage", cpuCurrent, "%", cpuTrend, cpuPrev, "cpu"))

	// 6. Memory Usage
	memTrend, err := repositories.MetricRecordRepository.GetAverageByHour(c, "ram", start, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	memCurrent := getLastValue(memTrend)
	memPrev, _ := repositories.MetricRecordRepository.GetAverageBetween(c, "ram", prevStart, prevEnd)
	metrics = append(metrics, buildMetric("memory_usage", "Memory Usage", memCurrent, "MB", memTrend, memPrev, "memory"))

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
		// Higher is worse for memory
		if value > 900 {
			return "critical"
		} else if value > 700 {
			return "warning"
		}
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
