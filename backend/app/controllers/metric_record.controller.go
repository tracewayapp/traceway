package controllers

import (
	"github.com/gin-gonic/gin"
)

type metricRecordController struct{}

func (e metricRecordController) FindHomepageStats(c *gin.Context) {
	// requests in last 24h vs previous 24h
	// exceptions in last 24h vs previous 24h
	// ram usage last 24h vs previous 24h
	// memory usage last 24h vs previous 24h

}

var MetricRecordController = metricRecordController{}
