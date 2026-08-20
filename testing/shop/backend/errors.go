package main

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func abortServerError(c *gin.Context, userMsg string, err error) {
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())
	slog.ErrorContext(c.Request.Context(), userMsg, "error", err.Error())
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": userMsg})
}

// recordServerError reports an error on the active span without aborting —
// for handled failure paths that still return a JSON error body themselves.
func recordServerError(c *gin.Context, err error) {
	span := trace.SpanFromContext(c.Request.Context())
	span.RecordError(err, trace.WithStackTrace(true))
	span.SetStatus(codes.Error, err.Error())
}
