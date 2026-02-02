package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	traceway "go.tracewayapp.com"
	tracewayfasthttp "go.tracewayapp.com/tracewayfasthttp"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
)

func innerFunction() error {
	return traceway.NewStackTraceErrorf("error from inner function")
}

func middleFunction() error {
	return innerFunction()
}

func outerFunction() error {
	return middleFunction()
}

func writeJSON(ctx *fasthttp.RequestCtx, status int, data interface{}) {
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(status)
	json.NewEncoder(ctx) .Encode(data)
}

func main() {
	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := router.New()

	r.GET("/test-ok", func(ctx *fasthttp.RequestCtx) {
		writeJSON(ctx, 200, map[string]string{"status": "ok"})
	})

	r.GET("/test-not-found", func(ctx *fasthttp.RequestCtx) {
		writeJSON(ctx, 404, map[string]string{"status": "not-found"})
	})

	r.GET("/test-exception", func(ctx *fasthttp.RequestCtx) {
		panic("test panic from /test-exception")
	})

	r.GET("/test-error-simple", func(ctx *fasthttp.RequestCtx) {
		traceway.CaptureException(errors.New("simple error without stack"))
		writeJSON(ctx, 500, map[string]string{"error": "simple error"})
	})

	r.GET("/test-error-stacktrace", func(ctx *fasthttp.RequestCtx) {
		err := traceway.NewStackTraceErrorf("error with stack trace")
		traceway.CaptureException(err)
		writeJSON(ctx, 500, map[string]string{"error": "stacktrace error"})
	})

	r.GET("/test-error-wrapped", func(ctx *fasthttp.RequestCtx) {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		traceway.CaptureException(wrapped2)
		writeJSON(ctx, 500, map[string]string{"error": "wrapped error"})
	})

	r.GET("/test-error-nested", func(ctx *fasthttp.RequestCtx) {
		err := outerFunction()
		traceway.CaptureException(err)
		writeJSON(ctx, 500, map[string]string{"error": "nested error"})
	})

	r.GET("/test-message", func(ctx *fasthttp.RequestCtx) {
		traceway.CaptureMessage("test message from /test-message")
		writeJSON(ctx, 200, map[string]string{"status": "message sent"})
	})

	r.GET("/test-message-attributes", func(ctx *fasthttp.RequestCtx) {
		traceway.CaptureMessageAttributes("test message with attributes", map[string]string{
			"source":   "test-message-attributes",
			"priority": "high",
		})
		writeJSON(ctx, 200, map[string]string{"status": "message with attributes sent"})
	})

	r.GET("/test-spans", func(ctx *fasthttp.RequestCtx) {
		bgCtx := context.Background()

		dbSpan := traceway.StartSpan(bgCtx, "db.query")
		time.Sleep(50 * time.Millisecond)
		dbSpan.End()

		cacheSpan := traceway.StartSpan(bgCtx, "cache.set")
		time.Sleep(20 * time.Millisecond)
		cacheSpan.End()

		httpSpan := traceway.StartSpan(bgCtx, "http.external_api")
		time.Sleep(100 * time.Millisecond)
		httpSpan.End()

		writeJSON(ctx, 200, map[string]string{"status": "spans captured"})
	})

	r.GET("/test-task", func(ctx *fasthttp.RequestCtx) {
		go func() {
			traceway.MeasureTask("background-data-processor", func(twctx context.Context) {
				span := traceway.StartSpan(twctx, "processing")
				time.Sleep(200 * time.Millisecond)
				span.End()
			})
		}()
		writeJSON(ctx, 200, map[string]string{"status": "task started"})
	})

	r.GET("/test-metric", func(ctx *fasthttp.RequestCtx) {
		traceway.CaptureMetric("test.custom_metric", 42.0)
		writeJSON(ctx, 200, map[string]string{"status": "metric captured"})
	})

	r.GET("/test-attributes", func(ctx *fasthttp.RequestCtx) {
		traceway.CaptureExceptionWithAttributes(
			errors.New("exception with custom attributes"),
			map[string]string{
				"user_id":    "usr_123",
				"request_id": "req_456",
				"env":        "testing",
			},
			nil,
		)
		writeJSON(ctx, 200, map[string]string{"status": "exception with attributes captured"})
	})

	r.POST("/test-recording", func(ctx *fasthttp.RequestCtx) {
		var body map[string]interface{}
		if err := json.Unmarshal(ctx.PostBody(), &body); err != nil {
			writeJSON(ctx, 400, map[string]string{"error": err.Error()})
			return
		}

		action, _ := body["action"].(string)
		if action == "panic" {
			panic("panic triggered by /test-recording")
		}

		writeJSON(ctx, 200, map[string]interface{}{"status": "ok", "received": body})
	})

	middleware := tracewayfasthttp.New(
		endpoint,
		tracewayfasthttp.WithDebug(true),
		tracewayfasthttp.WithOnErrorRecording(tracewayfasthttp.RecordingUrl|tracewayfasthttp.RecordingQuery|tracewayfasthttp.RecordingHeader|tracewayfasthttp.RecordingBody),
	)

	fmt.Printf("Fasthttp server starting on :%s\n", port)
	fasthttp.ListenAndServe(":"+port, middleware(r.Handler))
}
