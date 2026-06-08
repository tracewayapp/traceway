package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type endpointProbeResult struct {
	Name      string  `json:"name"`
	Path      string  `json:"path"`
	LatencyMs float64 `json:"latencyMs"`
	Ok        bool    `json:"ok"`
	Error     string  `json:"error,omitempty"`
}

type readProbeStep struct {
	FillLevelTarget      int64                 `json:"fillLevelTarget"`
	RowsIngested         int64                 `json:"rowsIngested"`
	IngestSecondsElapsed float64               `json:"ingestSecondsElapsed"`
	Probes               []endpointProbeResult `json:"probes"`
	MedianLatencyMs      float64               `json:"medianLatencyMs"`
	// Back-compat: equals MedianLatencyMs and AND-of-all-probe-Ok respectively.
	// Existing chart.py renderers (and post-1 readers) consume these.
	ReadLatencyMs float64 `json:"readLatencyMs"`
	ReadOk        bool    `json:"readOk"`
	Passed        bool    `json:"passed"`
	FailReason    string  `json:"failReason,omitempty"`
}

type readProbeEndpointDescriptor struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type readProbeResult struct {
	Endpoints []readProbeEndpointDescriptor `json:"endpoints"`
	// Back-compat: first endpoint's path. Older chart.py renderers read this.
	ReadPath           string          `json:"readPath"`
	ReadThresholdMs    int             `json:"readThresholdMs"`
	SettleSeconds      int             `json:"settleSeconds"`
	FillBatchSize      int             `json:"fillBatchSize"`
	FillRequestRate    float64         `json:"fillRequestRate"`
	Steps              []readProbeStep `json:"steps"`
	MaxFillLevelPassed int64           `json:"maxFillLevelPassed"`
}

// readProbeCheckpoint is invoked after every fill-level step so the caller
// can persist a partial report — we never want to lose progress when the
// process dies mid-run.
type readProbeCheckpoint func(readProbeResult)

// runReadProbe walks the configured fill levels. For each level it ingests
// rows until totalIngested >= target, settles for cfg.settleSeconds, then
// issues one read probe **per configured endpoint** for the signal, records
// per-endpoint latencies, and uses the median to pass/fail against
// cfg.readThresholdMs. Any HTTP error on any probe fails the step.
func runReadProbe(ctx context.Context, cfg config, ing *ingester, ingestStats *latencyTracker, client *http.Client, checkpoint readProbeCheckpoint) readProbeResult {
	endpoints := endpointSetForSignal(cfg.signal)
	descriptors := make([]readProbeEndpointDescriptor, len(endpoints))
	for i, ep := range endpoints {
		descriptors[i] = readProbeEndpointDescriptor{Name: ep.Name, Path: ep.Path}
	}
	res := readProbeResult{
		Endpoints:       descriptors,
		ReadThresholdMs: cfg.readThresholdMs,
		SettleSeconds:   int(cfg.settleSeconds.Seconds()),
		FillBatchSize:   cfg.fillBatchSize,
		FillRequestRate: cfg.fillRequestRate,
	}
	if len(endpoints) > 0 {
		res.ReadPath = endpoints[0].Path
	}

	ing.SetBatchSize(cfg.fillBatchSize)
	ing.SetRequestRate(cfg.fillRequestRate)

	// Reset the ingester counters before starting so totalIngested reflects
	// only what this scenario sent.
	ing.SnapshotAndResetItems()
	ingestStats.SnapshotAndReset()

	var totalIngested int64

	for _, target := range cfg.fillLevels {
		if ctx.Err() != nil {
			break
		}
		step := readProbeStep{FillLevelTarget: target}

		if totalIngested < target {
			fillStart := time.Now()
			ing.Start(ctx)
			pollFillProgress(ctx, ing, &totalIngested, target)
			ing.Stop()
			step.IngestSecondsElapsed = time.Since(fillStart).Seconds()
		}
		step.RowsIngested = totalIngested

		// Drain whatever bumped between Stop and Snapshot.
		extraAttempted, _ := ing.SnapshotAndResetItems()
		totalIngested += extraAttempted
		step.RowsIngested = totalIngested

		fmt.Fprintf(stderrPrefix(), "read-probe fill=%d rows reached in %.1fs (signal=%s) — settling %ds\n",
			step.RowsIngested, step.IngestSecondsElapsed, cfg.signal, res.SettleSeconds)

		select {
		case <-time.After(cfg.settleSeconds):
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			res.Steps = append(res.Steps, step)
			if checkpoint != nil {
				checkpoint(res)
			}
			break
		}

		probeResults := make([]endpointProbeResult, 0, len(endpoints))
		allOk := true
		latencies := make([]float64, 0, len(endpoints))
		for _, ep := range endpoints {
			latency, err := probeReadEndpoint(ctx, client, cfg, ep)
			pr := endpointProbeResult{
				Name:      ep.Name,
				Path:      ep.Path,
				LatencyMs: latency,
				Ok:        err == nil,
			}
			if err != nil {
				pr.Error = err.Error()
				allOk = false
			}
			probeResults = append(probeResults, pr)
			latencies = append(latencies, latency)
		}
		step.Probes = probeResults

		median := medianFloat(latencies)
		step.MedianLatencyMs = median
		step.ReadLatencyMs = median
		step.ReadOk = allOk

		switch {
		case !allOk:
			step.Passed = false
			// Find the first failed probe for the reason.
			for _, pr := range probeResults {
				if !pr.Ok {
					step.FailReason = fmt.Sprintf("probe %s failed: %s (latency %.0fms)", pr.Name, pr.Error, pr.LatencyMs)
					break
				}
			}
		case median > float64(cfg.readThresholdMs):
			step.Passed = false
			step.FailReason = fmt.Sprintf("median latency %.0fms > %dms threshold across %d probes", median, cfg.readThresholdMs, len(probeResults))
		default:
			step.Passed = true
			res.MaxFillLevelPassed = target
		}

		fmt.Fprintf(stderrPrefix(), "read-probe target=%d median=%.0fms probes=%d allOk=%t passed=%t %s\n",
			target, median, len(probeResults), allOk, step.Passed, step.FailReason)
		res.Steps = append(res.Steps, step)
		if checkpoint != nil {
			checkpoint(res)
		}
		if !step.Passed {
			break
		}
	}

	return res
}

// pollFillProgress drains attempted-item counters from the ingester until the
// target is reached or the context cancels. Sub-second polling keeps overshoot
// small (at fill-batch=8192 × 100 req/s the loadgen sends ~800k items/sec, so
// 200ms granularity overshoots by ≤160k items — negligible at 100M fill).
func pollFillProgress(ctx context.Context, ing *ingester, totalIngested *int64, target int64) {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	for *totalIngested < target {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			attempted, _ := ing.SnapshotAndResetItems()
			*totalIngested += attempted
		}
	}
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// readProbeEndpoint describes one HTTP probe target.
type readProbeEndpoint struct {
	Name  string
	Path  string
	Build func(cfg config, fromDate, toDate string, probeCtx context.Context) (*http.Request, error)
}

// endpointSetForSignal returns the 3 endpoints probed per fill-level for a
// given signal. Locked 2026-06-08 (see POSTS.md decision log). Order matters:
// the first entry's path becomes the back-compat readProbeResult.ReadPath.
func endpointSetForSignal(signal string) []readProbeEndpoint {
	switch signal {
	case "spans":
		return []readProbeEndpoint{
			{Name: "endpoints-grouped", Path: "/api/endpoints/grouped", Build: buildPostJSON("/api/endpoints/grouped", spansGroupedBody)},
			{Name: "endpoints-chart", Path: "/api/endpoints/chart", Build: buildPostJSON("/api/endpoints/chart", spansChartBody)},
			{Name: "exception-stack-traces", Path: "/api/exception-stack-traces", Build: buildPostJSON("/api/exception-stack-traces", spansExceptionsBody)},
		}
	case "metrics":
		return []readProbeEndpoint{
			{Name: "metrics-server", Path: "/api/metrics/server", Build: buildGetQuery("/api/metrics/server", metricsTimeRangeQuery)},
			{Name: "metrics-application", Path: "/api/metrics/application", Build: buildGetQuery("/api/metrics/application", metricsTimeRangeQuery)},
			{Name: "dashboard", Path: "/api/dashboard", Build: buildGetQuery("/api/dashboard", metricsTimeRangeQuery)},
		}
	case "logs":
		return []readProbeEndpoint{
			{Name: "logs-severity-error", Path: "/api/logs", Build: buildPostJSON("/api/logs", logsSeverityErrorBody)},
			{Name: "logs-body-search", Path: "/api/logs", Build: buildPostJSON("/api/logs", logsBodySearchBody)},
			{Name: "logs-trace-id", Path: "/api/logs", Build: buildPostJSON("/api/logs", logsTraceIdBody)},
		}
	}
	return nil
}

// probeReadEndpoint issues one HTTP request for the given endpoint and returns
// wall-clock latency in ms plus any error. Hard-capped at threshold + 1s so a
// hanging SUT can't deadlock the loop.
func probeReadEndpoint(ctx context.Context, client *http.Client, cfg config, ep readProbeEndpoint) (float64, error) {
	now := time.Now().UTC()
	fromDate := now.Add(-24 * time.Hour).Format(time.RFC3339)
	toDate := now.Format(time.RFC3339)

	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.readThresholdMs+1000)*time.Millisecond)
	defer cancel()

	req, err := ep.Build(cfg, fromDate, toDate, probeCtx)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.jwt)

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Seconds() * 1000
	if err != nil {
		return elapsed, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 400 {
		return elapsed, fmt.Errorf("status %d", resp.StatusCode)
	}
	return elapsed, nil
}

// buildPostJSON returns a request-builder that POSTs a JSON body. The body
// callback receives the date range so it can populate fromDate/toDate.
func buildPostJSON(path string, body func(fromDate, toDate string) map[string]any) func(cfg config, fromDate, toDate string, probeCtx context.Context) (*http.Request, error) {
	return func(cfg config, fromDate, toDate string, probeCtx context.Context) (*http.Request, error) {
		payload, err := json.Marshal(body(fromDate, toDate))
		if err != nil {
			return nil, err
		}
		u, err := url.Parse(cfg.target + path)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("projectId", cfg.projectId)
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, u.String(), bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		return req, nil
	}
}

// buildGetQuery returns a request-builder that GETs with extra query params.
func buildGetQuery(path string, query func(fromDate, toDate string) map[string]string) func(cfg config, fromDate, toDate string, probeCtx context.Context) (*http.Request, error) {
	return func(cfg config, fromDate, toDate string, probeCtx context.Context) (*http.Request, error) {
		u, err := url.Parse(cfg.target + path)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("projectId", cfg.projectId)
		for k, v := range query(fromDate, toDate) {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		return http.NewRequestWithContext(probeCtx, http.MethodGet, u.String(), nil)
	}
}

// --- Per-endpoint body / query builders ------------------------------------

func spansGroupedBody(fromDate, toDate string) map[string]any {
	return map[string]any{
		"fromDate":      fromDate,
		"toDate":        toDate,
		"orderBy":       "count",
		"sortDirection": "desc",
		"pagination":    map[string]int{"page": 1, "pageSize": 50},
		"search":        "",
	}
}

func spansChartBody(fromDate, toDate string) map[string]any {
	return map[string]any{
		"fromDate":        fromDate,
		"toDate":          toDate,
		"metricType":      "p95",
		"intervalMinutes": 5,
	}
}

func spansExceptionsBody(fromDate, toDate string) map[string]any {
	return map[string]any{
		"fromDate":        fromDate,
		"toDate":          toDate,
		"orderBy":         "count",
		"pagination":      map[string]int{"page": 1, "pageSize": 50},
		"search":          "",
		"searchType":      "message",
		"includeArchived": false,
	}
}

func metricsTimeRangeQuery(fromDate, toDate string) map[string]string {
	return map[string]string{
		"fromDate": fromDate,
		"toDate":   toDate,
	}
}

func logsSeverityErrorBody(fromDate, toDate string) map[string]any {
	// MinSeverity 17 = OTel SeverityNumber for ERROR.
	return map[string]any{
		"fromDate":      fromDate,
		"toDate":        toDate,
		"orderBy":       "timestamp",
		"sortDirection": "desc",
		"minSeverity":   17,
		"pagination":    map[string]int{"page": 1, "pageSize": 50},
	}
}

func logsBodySearchBody(fromDate, toDate string) map[string]any {
	// "error" is short, common, and inside the 24h unscoped-body-search
	// window allowed by /api/logs.
	return map[string]any{
		"fromDate":      fromDate,
		"toDate":        toDate,
		"orderBy":       "timestamp",
		"sortDirection": "desc",
		"search":        "error",
		"searchType":    "body",
		"pagination":    map[string]int{"page": 1, "pageSize": 50},
	}
}

func logsTraceIdBody(fromDate, toDate string) map[string]any {
	// Fixed dummy trace_id — the query exercises the trace_id index path;
	// result count doesn't matter for latency measurement.
	return map[string]any{
		"fromDate":      fromDate,
		"toDate":        toDate,
		"orderBy":       "timestamp",
		"sortDirection": "desc",
		"traceId":       "00000000000000000000000000000001",
		"pagination":    map[string]int{"page": 1, "pageSize": 50},
	}
}
