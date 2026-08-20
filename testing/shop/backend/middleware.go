package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, traceway-trace-id")
		c.Header("Access-Control-Expose-Headers", "traceway-trace-id")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Only /api requests should become endpoint spans; the embedded SPA assets
// would otherwise each get their own endpoint row.
func traceFilter(c *gin.Context) bool {
	path := c.Request.URL.Path
	return strings.HasPrefix(path, "/api") && path != "/api/health"
}

func otelRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("panic: %v", r)
				}
				span := trace.SpanFromContext(c.Request.Context())
				span.RecordError(err, trace.WithStackTrace(true))
				span.SetStatus(codes.Error, err.Error())
				slog.ErrorContext(c.Request.Context(), "panic recovered", "error", err.Error(), "path", c.Request.URL.Path)
				if strings.HasPrefix(c.Request.URL.Path, "/api/coupon") {
					recordCouponFailure(c.Request.Context())
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			}
		}()
		c.Next()
	}
}

func distributedTraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw := c.GetHeader("traceway-trace-id"); raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				trace.SpanFromContext(c.Request.Context()).
					SetAttributes(attribute.String("traceway.distributed_trace_id", id.String()))
				c.Header("traceway-trace-id", id.String())
			}
		}
		c.Next()
	}
}
