package controllers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tracewayapp/traceway/backend/app/db"
	"github.com/tracewayapp/traceway/backend/app/outbox"
	traceway "go.tracewayapp.com"
)

type healthDeepController struct{}

type TableParts struct {
	Table string `json:"table"`
	Parts int64  `json:"parts"`
}

type CHError struct {
	Name          string `json:"name"`
	Value         int64  `json:"value"`
	LastErrorTime string `json:"lastErrorTime,omitempty"`
}

type HealthDeepResponse struct {
	CHReachable      bool         `json:"chReachable"`
	CHUptimeSec      int64        `json:"chUptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []TableParts `json:"partsByTable,omitempty"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []CHError    `json:"errorsRecent,omitempty"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes,omitempty"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes,omitempty"`

	TelemetryBackend string                   `json:"telemetryBackend"`
	DroppedRows      map[string]uint64        `json:"droppedRows"`
	DroppedRowsTotal uint64                   `json:"droppedRowsTotal"`
	InsertFailures   uint64                   `json:"insertFailures"`
	IngestRejected   uint64                   `json:"ingestRejected"`
	Engine           *db.TelemetryEngineStats `json:"engine,omitempty"`
	Outbox           *outbox.HealthStats      `json:"outbox,omitempty"`
}

// 503 means the configured telemetry backend is down. On the embedded
// backends chReachable stays false with a 200, which is what the loadgen's
// merge-idle fast path keys on.
func (h healthDeepController) Get(c *gin.Context) {
	resp := fetchCHHealth(c.Request.Context())

	dropped, droppedTotal, insertFailures := db.GetTelemetryIngestCounters()
	resp.TelemetryBackend = db.TelemetryBackendName()
	resp.DroppedRows = dropped
	resp.DroppedRowsTotal = droppedTotal
	resp.InsertFailures = insertFailures
	resp.IngestRejected = db.GetIngestRejects()
	if engine, ok := db.GetTelemetryEngineStats(c.Request.Context()); ok {
		resp.Engine = &engine
	}
	// Best-effort: health must not 500 because the main DB hiccuped.
	if outboxStats, err := outbox.HealthSnapshot(); err == nil {
		resp.Outbox = outboxStats
	} else {
		traceway.CaptureException(fmt.Errorf("failed to load outbox health: %w", err))
	}

	if resp.TelemetryBackend == "clickhouse" && !resp.CHReachable {
		c.JSON(http.StatusServiceUnavailable, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

var HealthDeepController = healthDeepController{}
