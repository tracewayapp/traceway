package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"

	traceway "go.tracewayapp.com"
	tracewaygin "go.tracewayapp.com/traceway_gin"

	"github.com/gin-gonic/gin"
)

type CustomError struct {
	Code    int
	Message string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("CustomError[%d]: %s", e.Code, e.Message)
}

func innerFunction() error {
	return traceway.NewStackTraceError("error from inner function", 0)
}

func middleFunction() error {
	return innerFunction()
}

func outerFunction() error {
	return middleFunction()
}

func main() {
	testGin()
}

type JsonRecordingTest struct {
	Name string
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
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))

	router.POST("/test-recording/:param", func(ctx *gin.Context) {
		var data JsonRecordingTest

		if err := ctx.ShouldBindJSON(&data); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.Name != "good" {
			panic("Bad") // lol
		}
	})

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

	router.GET("/test-cerror-simple", func(ctx *gin.Context) {
		ctx.Error(errors.New("simple error without stack"))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "simple error"})
	})

	router.GET("/test-cerror-wrapped", func(ctx *gin.Context) {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		ctx.Error(wrapped2)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped error"})
	})

	router.GET("/test-cerror-stacktrace", func(ctx *gin.Context) {
		err := traceway.NewStackTraceError("error with stack trace", 0)
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "stacktrace error"})
	})

	router.GET("/test-cerror-stacktrace-wrapped", func(ctx *gin.Context) {
		base := traceway.NewStackTraceError("base error with stack", 0)
		wrapped := fmt.Errorf("wrapped with fmt: %w", base)
		ctx.Error(wrapped)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped stacktrace error"})
	})

	router.GET("/test-cerror-multiple", func(ctx *gin.Context) {
		ctx.Error(errors.New("first error"))
		ctx.Error(traceway.NewStackTraceError("second error with stack", 0))
		ctx.Error(fmt.Errorf("third error: %w", errors.New("nested")))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "multiple errors"})
	})

	router.GET("/test-cerror-nested", func(ctx *gin.Context) {
		err := outerFunction()
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "nested function error"})
	})

	router.GET("/test-cerror-custom", func(ctx *gin.Context) {
		err := &CustomError{Code: 500, Message: "something went wrong"}
		ctx.Error(err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "custom error"})
	})

	router.Run()
}

var veryLongJsonForTestin = `{"str": "traceway", "obj": {"id": 1}, "obj2": {"id": 1}, "obj3": {"id": 1}, "obj4": {"id": 1}, "obj5loremipsumdoloret": {"id": "I'm baby tumeric VHS Brooklyn, echo park literally you probably haven't heard of them crucifix taiyaki chambray roof party man bun knausgaard waistcoat squid health goth. Gastropub godard bodega boys snackwave asymmetrical la croix. Whatever try-hard pour-over humblebrag austin microdosing organic bruh. Keffiyeh mukbang yuccie, 90's humblebrag roof party godard kale chips lo-fi sriracha aesthetic.", "id2": "ImbabytumericVHSBrooklynechoparkliterallyyouprobablyhaventheardofthemcrucifixtaiyakichambrayroofpartymanbunknausgaardwaistcoatsquidhealthgothGastropubgodardbodegaboyssnackwaveasymmetricallacroixWhatevertryhardpouroverhumblebragaustinmicrodosingorganicbruhKeffiyehmukbangyuccieshumblebragroofpartygodardkalechipslofisrirachaaesthetic"}, "arr": [1, 2, "", {"key": 1, "key2": "example"}]}`
