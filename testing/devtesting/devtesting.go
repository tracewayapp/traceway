package main

import (
	"context"
	"errors"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	traceway "go.tracewayapp.com"
	tracewaygin "go.tracewayapp.com/traceway_gin"

	"github.com/gin-gonic/gin"
)

func main() {
	testGin()
}

func testGin() {
	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	router := gin.Default()

	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		tracewaygin.WithRepanic(true),
	))

	router.GET("/test-task", func(ctx *gin.Context) {
		go func() {
			traceway.MeasureTask("traceway data processor", func(twctx context.Context) {
				seg := traceway.StartSegment(twctx, "loading data")
				time.Sleep(time.Second * time.Duration(rand.Float64()*2))
				seg.End()

				for i := range 10 {
					traceway.CaptureMessageWithContext(twctx, "data loaded successfully "+strconv.Itoa(i))
				}

				traceway.CaptureExceptionWithContext(twctx, errors.New("what an error"))
			})
		}()
	})
	router.GET("/test-json", func(ctx *gin.Context) {
		scope := traceway.GetScopeFromContext(ctx)
		scope.SetTag("json tag", veryLongJsonForTestin)
		traceway.CaptureMessageWithContext(ctx, "test json")
	})

	router.GET("/test-message", func(ctx *gin.Context) {
		for i := range 10 {
			traceway.CaptureMessageWithContext(ctx, "test message "+strconv.Itoa(i))
		}

		traceway.CaptureExceptionWithContext(ctx, errors.New("test message exception"))
	})

	router.GET("/test-50k", func(ctx *gin.Context) {
		for i := range 50_000 {
			traceway.CaptureMessage("I:" + strconv.Itoa(i))
		}
	})

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

var veryLongJsonForTestin = `{"str": "traceway", "obj": {"id": 1}, "obj2": {"id": 1}, "obj3": {"id": 1}, "obj4": {"id": 1}, "obj5loremipsumdoloret": {"id": "I'm baby tumeric VHS Brooklyn, echo park literally you probably haven't heard of them crucifix taiyaki chambray roof party man bun knausgaard waistcoat squid health goth. Gastropub godard bodega boys snackwave asymmetrical la croix. Whatever try-hard pour-over humblebrag austin microdosing organic bruh. Keffiyeh mukbang yuccie, 90's humblebrag roof party godard kale chips lo-fi sriracha aesthetic.", "id2": "ImbabytumericVHSBrooklynechoparkliterallyyouprobablyhaventheardofthemcrucifixtaiyakichambrayroofpartymanbunknausgaardwaistcoatsquidhealthgothGastropubgodardbodegaboyssnackwaveasymmetricallacroixWhatevertryhardpouroverhumblebragaustinmicrodosingorganicbruhKeffiyehmukbangyuccieshumblebragroofpartygodardkalechipslofisrirachaaesthetic"}, "arr": [1, 2, "", {"key": 1, "key2": "example"}]}`
