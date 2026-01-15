package controllers

import (
	"backend/app/models"
	"backend/app/repositories"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type metricRecordController struct{}

type HomepageStatsRequest struct {
	ProjectId string `json:"projectId"`
}

func (e metricRecordController) FindHomepageStats(c *gin.Context) {
	var request HomepageStatsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	projectId := request.ProjectId

	// requests in last 24h vs previous 24h
	now := time.Now()
	oneDayAgo := now.Add(-24 * time.Hour)
	twoDaysAgo := now.Add(-48 * time.Hour)

	// requests
	requestsNow, err := repositories.EndpointRepository.CountBetween(c, projectId, oneDayAgo, now)
	if err != nil {
		panic(err)
	}
	requestsPrev, err := repositories.EndpointRepository.CountBetween(c, projectId, twoDaysAgo, oneDayAgo)
	if err != nil {
		panic(err)
	}

	// exceptions
	exceptionsNow, err := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, oneDayAgo, now)
	if err != nil {
		panic(err)
	}
	exceptionsPrev, err := repositories.ExceptionStackTraceRepository.CountBetween(c, projectId, twoDaysAgo, oneDayAgo)
	if err != nil {
		panic(err)
	}

	// ram usage last 24h vs previous 24h
	ramNow, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, oneDayAgo, now)
	if err != nil {
		panic(err)
	}
	ramPrev, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameMemoryUsage, twoDaysAgo, oneDayAgo)
	if err != nil {
		panic(err)
	}

	// memory usage last 24h vs previous 24h
	cpuNow, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, oneDayAgo, now)
	if err != nil {
		panic(err)
	}
	cpuPrev, err := repositories.MetricRecordRepository.GetAverageBetween(c, projectId, models.MetricNameCpuUsage, twoDaysAgo, oneDayAgo)
	if err != nil {
		panic(err)
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
