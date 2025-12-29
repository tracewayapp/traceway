package controllers

import (
	"backend/app/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterControllers(router *gin.RouterGroup) {
	router.POST("/report", middleware.UseAuth, middleware.UseGzip, middleware.UseTransaction, ClientController.Report)
}
