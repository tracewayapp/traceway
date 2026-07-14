package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type tableParts struct {
	Table string `json:"table"`
	Parts int64  `json:"parts"`
}

type chError struct {
	Name          string `json:"name"`
	Value         int64  `json:"value"`
	LastErrorTime string `json:"lastErrorTime,omitempty"`
}

type chSnapshot struct {
	Reachable        bool         `json:"reachable"`
	UptimeSec        int64        `json:"uptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []tableParts `json:"partsByTable,omitempty"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []chError    `json:"errorsRecent,omitempty"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes,omitempty"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes,omitempty"`
}

// ingestStatsSnapshot carries the SUT-side write-path counters from
// /health/deep: silently dropped rows, batch insert failures, and on DuckDB
// engine gauges (db/WAL file size, memory, read-pool waits). Reachable=false
// (missing or unparseable response) makes the caller skip drop-delta
// evaluation rather than fail the step. Older SUT images without these
// fields decode to zeros, which keeps the drop criterion inert.
type ingestStatsSnapshot struct {
	Reachable        bool  `json:"reachable"`
	DroppedRowsTotal int64 `json:"droppedRowsTotal"`
	InsertFailures   int64 `json:"insertFailures"`
	DBSizeBytes      int64 `json:"dbSizeBytes,omitempty"`
	WALSizeBytes     int64 `json:"walSizeBytes,omitempty"`
	MemoryUsedBytes  int64 `json:"memoryUsedBytes,omitempty"`
}

// healthDeepBody mirrors the backend HealthDeepResponse with its JSON tags. The
// backend uses `chReachable`/`chUptimeSec`; we expose those as `reachable`/
// `uptimeSec` in our embedded snapshot so the bench JSON stays consistent with
// other loadgen fields.
type healthDeepBody struct {
	CHReachable      bool         `json:"chReachable"`
	CHUptimeSec      int64        `json:"chUptimeSec"`
	PartsCount       int64        `json:"partsCount"`
	PartsByTable     []tableParts `json:"partsByTable"`
	ActiveMerges     int64        `json:"activeMerges"`
	LongestMergeSec  float64      `json:"longestMergeSec"`
	ErrorsRecent     []chError    `json:"errorsRecent"`
	MemoryUsageBytes int64        `json:"memoryUsageBytes"`
	MemoryTotalBytes int64        `json:"memoryTotalBytes"`
	DroppedRowsTotal int64        `json:"droppedRowsTotal"`
	InsertFailures   int64        `json:"insertFailures"`
	Engine           *struct {
		DBSizeBytes     int64 `json:"dbSizeBytes"`
		WALSizeBytes    int64 `json:"walSizeBytes"`
		MemoryUsedBytes int64 `json:"memoryUsedBytes"`
	} `json:"engine"`
}

// fetchDeepHealth pings the backend's /health/deep endpoint and splits the
// payload into the ClickHouse snapshot and the write-path ingest stats. On any
// error (timeout, transport, unexpected status, decode) it returns zero-value
// snapshots with Reachable=false and logs to stderrPrefix() — the caller does
// NOT fail the step on a missing snapshot. A 503 from the backend (CH
// telemetry configured but unreachable) is still parsed and embedded so the
// JSON captures the chReachable=false signal.
func fetchDeepHealth(ctx context.Context, cfg config, client *http.Client) (chSnapshot, ingestStatsSnapshot) {
	if cfg.jwt == "" {
		return chSnapshot{}, ingestStatsSnapshot{}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, cfg.target+"/api/health/deep", nil)
	if err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchDeepHealth: build request failed: %v\n", err)
		return chSnapshot{}, ingestStatsSnapshot{}
	}
	req.Header.Set("Authorization", "Bearer "+cfg.jwt)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchDeepHealth: http error: %v\n", err)
		return chSnapshot{}, ingestStatsSnapshot{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
		fmt.Fprintf(stderrPrefix(), "fetchDeepHealth: unexpected status %d\n", resp.StatusCode)
		return chSnapshot{}, ingestStatsSnapshot{}
	}

	var body healthDeepBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		fmt.Fprintf(stderrPrefix(), "fetchDeepHealth: decode failed: %v\n", err)
		return chSnapshot{}, ingestStatsSnapshot{}
	}

	ch := chSnapshot{
		Reachable:        body.CHReachable,
		UptimeSec:        body.CHUptimeSec,
		PartsCount:       body.PartsCount,
		PartsByTable:     body.PartsByTable,
		ActiveMerges:     body.ActiveMerges,
		LongestMergeSec:  body.LongestMergeSec,
		ErrorsRecent:     body.ErrorsRecent,
		MemoryUsageBytes: body.MemoryUsageBytes,
		MemoryTotalBytes: body.MemoryTotalBytes,
	}
	ingest := ingestStatsSnapshot{
		Reachable:        true,
		DroppedRowsTotal: body.DroppedRowsTotal,
		InsertFailures:   body.InsertFailures,
	}
	if body.Engine != nil {
		ingest.DBSizeBytes = body.Engine.DBSizeBytes
		ingest.WALSizeBytes = body.Engine.WALSizeBytes
		ingest.MemoryUsedBytes = body.Engine.MemoryUsedBytes
	}
	return ch, ingest
}
