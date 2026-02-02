package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	traceway "go.tracewayapp.com"
	tracewaygin "go.tracewayapp.com/tracewaygin"

	"github.com/gin-gonic/gin"
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

func main() {
	endpoint := os.Getenv("TRACEWAY_ENDPOINT")
	if endpoint == "" {
		endpoint = "default_token_change_me@http://localhost:8082/api/report"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	router := gin.Default()

	router.Use(tracewaygin.New(
		endpoint,
		tracewaygin.WithDebug(true),
		tracewaygin.WithOnErrorRecording(tracewaygin.RecordingUrl|tracewaygin.RecordingQuery|tracewaygin.RecordingHeader|tracewaygin.RecordingBody),
	))

	router.GET("/test-ok", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	router.GET("/test-not-found", func(c *gin.Context) {
		c.JSON(404, gin.H{"status": "not-found"})
	})

	router.GET("/test-exception", func(c *gin.Context) {
		panic("test panic from /test-exception")
	})

	router.GET("/test-error-simple", func(c *gin.Context) {
		c.Error(errors.New("simple error without stack"))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "simple error"})
	})

	router.GET("/test-error-stacktrace", func(c *gin.Context) {
		err := traceway.NewStackTraceErrorf("error with stack trace")
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stacktrace error"})
	})

	router.GET("/test-error-wrapped", func(c *gin.Context) {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		c.Error(wrapped2)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "wrapped error"})
	})

	router.GET("/test-error-nested", func(c *gin.Context) {
		err := outerFunction()
		c.Error(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "nested error"})
	})

	router.GET("/test-message", func(c *gin.Context) {
		traceway.CaptureMessageWithContext(c, "test message from /test-message")
		c.JSON(200, gin.H{"status": "message sent"})
	})

	router.GET("/test-message-attributes", func(c *gin.Context) {
		traceway.CaptureMessageAttributes("test message with attributes", map[string]string{
			"source":   "test-message-attributes",
			"priority": "high",
		})
		c.JSON(200, gin.H{"status": "message with attributes sent"})
	})

	router.GET("/test-spans", func(c *gin.Context) {
		dbSpan := traceway.StartSpan(c, "db.query")
		time.Sleep(50 * time.Millisecond)
		dbSpan.End()

		cacheSpan := traceway.StartSpan(c, "cache.set")
		time.Sleep(20 * time.Millisecond)
		cacheSpan.End()

		httpSpan := traceway.StartSpan(c, "http.external_api")
		time.Sleep(100 * time.Millisecond)
		httpSpan.End()

		c.JSON(200, gin.H{"status": "spans captured"})
	})

	router.GET("/test-task", func(c *gin.Context) {
		go func() {
			traceway.MeasureTask("background-data-processor", func(twctx context.Context) {
				span := traceway.StartSpan(twctx, "processing")
				time.Sleep(200 * time.Millisecond)
				span.End()
			})
		}()
		c.JSON(200, gin.H{"status": "task started"})
	})

	router.GET("/test-metric", func(c *gin.Context) {
		traceway.CaptureMetric("test.custom_metric", 42.0)
		c.JSON(200, gin.H{"status": "metric captured"})
	})

	router.GET("/test-attributes", func(c *gin.Context) {
		traceway.CaptureExceptionWithAttributes(
			errors.New("exception with custom attributes"),
			map[string]string{
				"user_id":    "usr_123",
				"request_id": "req_456",
				"env":        "testing",
			},
			nil,
		)
		c.JSON(200, gin.H{"status": "exception with attributes captured"})
	})

	router.POST("/test-recording", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		action, _ := body["action"].(string)
		if action == "panic" {
			panic("panic triggered by /test-recording")
		}

		raw, _ := json.Marshal(body)
		c.JSON(200, gin.H{"status": "ok", "received": json.RawMessage(raw)})
	})

	router.Run(":" + port)
}
