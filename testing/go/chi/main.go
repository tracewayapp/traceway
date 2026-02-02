package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	traceway "go.tracewayapp.com"
	tracewaychi "go.tracewayapp.com/tracewaychi"

	"github.com/go-chi/chi/v5"
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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

	r := chi.NewRouter()

	r.Use(tracewaychi.New(
		endpoint,
		tracewaychi.WithDebug(true),
		tracewaychi.WithOnErrorRecording(tracewaychi.RecordingUrl|tracewaychi.RecordingQuery|tracewaychi.RecordingHeader|tracewaychi.RecordingBody),
	))

	r.Get("/test-ok", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	r.Get("/test-not-found", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]string{"status": "not-found"})
	})

	r.Get("/test-exception", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic from /test-exception")
	})

	r.Get("/test-error-simple", func(w http.ResponseWriter, r *http.Request) {
		traceway.CaptureExceptionWithContext(r.Context(), errors.New("simple error without stack"))
		writeJSON(w, 500, map[string]string{"error": "simple error"})
	})

	r.Get("/test-error-stacktrace", func(w http.ResponseWriter, r *http.Request) {
		err := traceway.NewStackTraceErrorf("error with stack trace")
		traceway.CaptureExceptionWithContext(r.Context(), err)
		writeJSON(w, 500, map[string]string{"error": "stacktrace error"})
	})

	r.Get("/test-error-wrapped", func(w http.ResponseWriter, r *http.Request) {
		base := errors.New("base error")
		wrapped := fmt.Errorf("layer 1: %w", base)
		wrapped2 := fmt.Errorf("layer 2: %w", wrapped)
		traceway.CaptureExceptionWithContext(r.Context(), wrapped2)
		writeJSON(w, 500, map[string]string{"error": "wrapped error"})
	})

	r.Get("/test-error-nested", func(w http.ResponseWriter, r *http.Request) {
		err := outerFunction()
		traceway.CaptureExceptionWithContext(r.Context(), err)
		writeJSON(w, 500, map[string]string{"error": "nested error"})
	})

	r.Get("/test-message", func(w http.ResponseWriter, r *http.Request) {
		traceway.CaptureMessageWithContext(r.Context(), "test message from /test-message")
		writeJSON(w, 200, map[string]string{"status": "message sent"})
	})

	r.Get("/test-message-attributes", func(w http.ResponseWriter, r *http.Request) {
		traceway.CaptureMessageAttributes("test message with attributes", map[string]string{
			"source":   "test-message-attributes",
			"priority": "high",
		})
		writeJSON(w, 200, map[string]string{"status": "message with attributes sent"})
	})

	r.Get("/test-spans", func(w http.ResponseWriter, r *http.Request) {
		dbSpan := traceway.StartSpan(r.Context(), "db.query")
		time.Sleep(50 * time.Millisecond)
		dbSpan.End()

		cacheSpan := traceway.StartSpan(r.Context(), "cache.set")
		time.Sleep(20 * time.Millisecond)
		cacheSpan.End()

		httpSpan := traceway.StartSpan(r.Context(), "http.external_api")
		time.Sleep(100 * time.Millisecond)
		httpSpan.End()

		writeJSON(w, 200, map[string]string{"status": "spans captured"})
	})

	r.Get("/test-task", func(w http.ResponseWriter, r *http.Request) {
		go func() {
			traceway.MeasureTask("background-data-processor", func(twctx context.Context) {
				span := traceway.StartSpan(twctx, "processing")
				time.Sleep(200 * time.Millisecond)
				span.End()
			})
		}()
		writeJSON(w, 200, map[string]string{"status": "task started"})
	})

	r.Get("/test-metric", func(w http.ResponseWriter, r *http.Request) {
		traceway.CaptureMetric("test.custom_metric", 42.0)
		writeJSON(w, 200, map[string]string{"status": "metric captured"})
	})

	r.Get("/test-attributes", func(w http.ResponseWriter, r *http.Request) {
		traceway.CaptureExceptionWithAttributes(
			errors.New("exception with custom attributes"),
			map[string]string{
				"user_id":    "usr_123",
				"request_id": "req_456",
				"env":        "testing",
			},
			nil,
		)
		writeJSON(w, 200, map[string]string{"status": "exception with attributes captured"})
	})

	r.Post("/test-recording", func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		defer r.Body.Close()

		var body map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}

		action, _ := body["action"].(string)
		if action == "panic" {
			panic("panic triggered by /test-recording")
		}

		writeJSON(w, 200, map[string]interface{}{"status": "ok", "received": body})
	})

	fmt.Printf("Chi server starting on :%s\n", port)
	http.ListenAndServe(":"+port, r)
}
