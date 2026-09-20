package controllers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/middleware"
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	traceway "go.tracewayapp.com"
	"google.golang.org/protobuf/proto"
)

func GetOtelSpan(c *gin.Context) {
	project, err := middleware.GetProjectId(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("RequireProjectAccess middleware must be applied: %w", err))
		return
	}
	traceID, spanID := strings.ToLower(c.Param("traceId")), strings.ToLower(c.Param("spanId"))
	if !validTraceHex(traceID) || !validSpanHex(spanID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTLP trace/span ID"})
		return
	}
	at, err := time.Parse(time.RFC3339Nano, c.Query("at"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "An RFC 3339 `at` time is required: span reads are held to 24 hours either side of it"})
		return
	}
	payload, err := telemetry.OtelSpanRepository.FindOTLP(c, project, traceID, spanID, at)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error loading stored OTLP span: %w", err))
		return
	}
	if len(payload) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Span not found"})
		return
	}
	var resource tracepb.ResourceSpans
	if err := proto.Unmarshal(payload, &resource); err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("stored OTLP span payload is not a valid ResourceSpans: %w", err))
		return
	}
	response, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{&resource}})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, traceway.NewStackTraceErrorf("error encoding OTLP span response: %w", err))
		return
	}
	c.Data(http.StatusOK, "application/x-protobuf", response)
}
