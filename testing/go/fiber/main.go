package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	traceway "go.tracewayapp.com"
	tracewayfiber "go.tracewayapp.com/tracewayfiber"

	"github.com/gofiber/fiber/v2"
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

	app := fiber.New()

	app.Use(tracewayfiber.New(
		endpoint,
		tracewayfiber.WithDebug(true),
		tracewayfiber.WithOnErrorRecording(tracewayfiber.RecordingUrl|tracewayfiber.RecordingQuery|tracewayfiber.RecordingHeader|tracewayfiber.RecordingBody),
	))

	app.Get("/test-ok", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/test-not-found", func(c *fiber.Ctx) error {
		c.Status(404)
		return c.JSON(fiber.Map{"status": "not-found"})
	})

	app.Get("/test-exception", func(c *fiber.Ctx) error {
		panic("test panic from /test-exception")
	})

	app.Get("/test-error-simple", func(c *fiber.Ctx) error {
		traceway.CaptureException(errors.New("simple error without stack"))
		c.Status(500)
		return c.JSON(fiber.Map{"error": "simple error"})
	})

	app.Get("/test-error-stacktrace", func(c *fiber.Ctx) error {
		err := traceway.NewStackTraceErrorf("error with stack trace")
		traceway.CaptureException(err)
		c.Status(500)
		return c.JSON(fiber.Map{"error": "stacktrace error"})
	})

	app.Get("/test-error-wrapped", func(c *fiber.Ctx) error {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		traceway.CaptureException(wrapped2)
		c.Status(500)
		return c.JSON(fiber.Map{"error": "wrapped error"})
	})

	app.Get("/test-error-nested", func(c *fiber.Ctx) error {
		err := outerFunction()
		traceway.CaptureException(err)
		c.Status(500)
		return c.JSON(fiber.Map{"error": "nested error"})
	})

	app.Get("/test-message", func(c *fiber.Ctx) error {
		traceway.CaptureMessage("test message from /test-message")
		return c.JSON(fiber.Map{"status": "message sent"})
	})

	app.Get("/test-message-attributes", func(c *fiber.Ctx) error {
		traceway.CaptureMessageAttributes("test message with attributes", map[string]string{
			"source":   "test-message-attributes",
			"priority": "high",
		})
		return c.JSON(fiber.Map{"status": "message with attributes sent"})
	})

	app.Get("/test-spans", func(c *fiber.Ctx) error {
		ctx := context.Background()

		dbSpan := traceway.StartSpan(ctx, "db.query")
		time.Sleep(50 * time.Millisecond)
		dbSpan.End()

		cacheSpan := traceway.StartSpan(ctx, "cache.set")
		time.Sleep(20 * time.Millisecond)
		cacheSpan.End()

		httpSpan := traceway.StartSpan(ctx, "http.external_api")
		time.Sleep(100 * time.Millisecond)
		httpSpan.End()

		return c.JSON(fiber.Map{"status": "spans captured"})
	})

	app.Get("/test-task", func(c *fiber.Ctx) error {
		go func() {
			traceway.MeasureTask("background-data-processor", func(twctx context.Context) {
				span := traceway.StartSpan(twctx, "processing")
				time.Sleep(200 * time.Millisecond)
				span.End()
			})
		}()
		return c.JSON(fiber.Map{"status": "task started"})
	})

	app.Get("/test-metric", func(c *fiber.Ctx) error {
		traceway.CaptureMetric("test.custom_metric", 42.0)
		return c.JSON(fiber.Map{"status": "metric captured"})
	})

	app.Get("/test-attributes", func(c *fiber.Ctx) error {
		traceway.CaptureExceptionWithAttributes(
			errors.New("exception with custom attributes"),
			map[string]string{
				"user_id":    "usr_123",
				"request_id": "req_456",
				"env":        "testing",
			},
			nil,
		)
		return c.JSON(fiber.Map{"status": "exception with attributes captured"})
	})

	app.Post("/test-recording", func(c *fiber.Ctx) error {
		var body map[string]interface{}
		if err := c.BodyParser(&body); err != nil {
			c.Status(400)
			return c.JSON(fiber.Map{"error": err.Error()})
		}

		action, _ := body["action"].(string)
		if action == "panic" {
			panic("panic triggered by /test-recording")
		}

		return c.JSON(fiber.Map{"status": "ok", "received": body})
	})

	fmt.Printf("Fiber server starting on :%s\n", port)
	app.Listen(":" + port)
}
