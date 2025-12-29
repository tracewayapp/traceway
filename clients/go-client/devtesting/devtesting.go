package main

import (
	"traceway"
	tracewaygin "traceway/traceway_gin"

	"github.com/gin-gonic/gin"
)

func main() {
	testGin()
}

func testGin() {
	router := gin.Default()

	router.Use(tracewaygin.New("demotoken@localhost:8081/report"))

	router.GET("/test-exception", func(ctx *gin.Context) {
		panic("Cool")
	})

	router.GET("/metrics", func(ctx *gin.Context) {
		traceway.PrintCollectionFrameMetrics()
	})

	router.Run()
}
