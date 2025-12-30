package controllers

import (
	"backend/app/controllers/clientcontrollers"
	"backend/app/middleware"

	"github.com/gin-gonic/gin"
)

type PaginationParams struct {
	Page     int `form:"page" binding:"min=1"`
	PageSize int `form:"page_size" binding:"min=1,max=100"`
}

type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

func RegisterControllers(router *gin.RouterGroup) {
	router.POST("/report", middleware.UseClientAuth, middleware.UseGzip, clientcontrollers.ClientController.Report)

	router.POST("/stats", middleware.UseAppAuth, middleware.UseGzip, MetricRecordController.FindHomepageStats)

	// router.POST("/transactions", middleware.UseAppAuth, middleware.UseGzip, TransactionController.FindAllTransactions)
	// router.POST("/exception-stack-traces", middleware.UseAppAuth, middleware.UseGzip, AppController.FindAllExceptionStackTraces)
}
