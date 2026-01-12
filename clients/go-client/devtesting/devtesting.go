package main

import (
	"errors"
	"math/rand/v2"
	"time"
	"traceway"
	tracewaygin "traceway/traceway_gin"

	"github.com/gin-gonic/gin"
)

func main() {
	testGin()
}

func testGin() {

	router := gin.Default()

	router.Use(tracewaygin.New(
		"default_token_change_me@http://localhost:8082/api/report",
		traceway.WithDebug(true),
	))

	router.GET("/test-exception", func(ctx *gin.Context) {
		time.Sleep(time.Duration(rand.IntN(2000)) * time.Millisecond)
		panic("Cool")
	})

	router.GET("/test-self-report-scope", func(ctx *gin.Context) {
		traceway.CaptureExceptionWithScope(errors.New("Test"), map[string]string{"Cool": "Pretty cool"}, nil)
	})

	router.GET("/test-self-report-context", func(ctx *gin.Context) {
		scope := traceway.GetScopeFromContext(ctx)
		scope.SetTag("Interesting", "Pretty Cool")

		traceway.CaptureExceptionWithContext(ctx, errors.New("Test2"))
	})

	router.GET("/test-ok", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"status": "ok",
		})
	})
	router.GET("/test-not-found", func(ctx *gin.Context) {
		ctx.JSON(404, gin.H{
			"status": "not-found",
		})
	})

	router.GET("/test-param/:param", func(ctx *gin.Context) {
		ctx.JSON(200, gin.H{
			"param": ctx.Param("param"),
		})
	})

	// Example: Capturing segments within a transaction
	router.GET("/test-segments", func(ctx *gin.Context) {
		dbAndCacheSeg := traceway.StartSegment(ctx, "db.and.cache")

		// Start a segment for database operation
		seg := traceway.StartSegment(ctx, "db.query")
		time.Sleep(time.Duration(50+rand.IntN(100)) * time.Millisecond)
		seg.End()

		// Start a segment for cache operation
		seg = traceway.StartSegment(ctx, "cache.set")
		time.Sleep(time.Duration(10+rand.IntN(30)) * time.Millisecond)
		seg.End()

		// Start a segment for an HTTP call
		seg = traceway.StartSegment(ctx, "http.external_api")
		time.Sleep(time.Duration(100+rand.IntN(200)) * time.Millisecond)
		seg.End()

		dbAndCacheSeg.End()

		ctx.JSON(200, gin.H{
			"status":  "ok",
			"message": "Segments captured",
		})
	})

	router.GET("/metrics", func(ctx *gin.Context) {
		traceway.PrintCollectionFrameMetrics()
	})

	router.Run()
}
