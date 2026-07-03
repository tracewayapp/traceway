package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// signalSender abstracts per-signal payload generation. The ingester does the
// HTTP plumbing once and delegates only the signal-specific bits — building the
// protobuf body and parsing the partial-success response — to implementations.
type signalSender interface {
	Name() string
	// Path is the OTLP-standard signal suffix (/v1/traces etc); the full URL is
	// target + --otlp-path-prefix + Path, so the same senders work against
	// Traceway (/api/otel) and standalone collectors like VictoriaMetrics
	// (/opentelemetry).
	Path() string
	BuildBody(rng *rand.Rand, batchSize int) ([]byte, error)
	ParseRejected(respBody []byte) int
}

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func intAttr(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func randomString(rng *rand.Rand, n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[rng.Intn(len(charset))]
	}
	return string(b)
}

// sendOneOTLP performs one POST cycle for a generic signalSender. The ingester
// calls this in its worker loop. attemptedItems is bumped before the request
// goes out so that drops on the wire still count toward "attempted"; rejected
// (server-reported partial-success) is bumped only on successful 2xx.
func sendOneOTLP(
	ctx context.Context,
	client *http.Client,
	cfg config,
	sender signalSender,
	rng *rand.Rand,
	batchSize int,
	stats *latencyTracker,
	attemptedItems *atomic.Int64,
	rejectedItems *atomic.Int64,
) {
	body, err := sender.BuildBody(rng, batchSize)
	if err != nil {
		stats.Record(0, err)
		return
	}
	gz, err := gzipCompress(body)
	if err != nil {
		stats.Record(0, err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.target+cfg.otlpPathPrefix+sender.Path(), bytes.NewReader(gz))
	if err != nil {
		stats.Record(0, err)
		return
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "gzip")
	if cfg.projectToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.projectToken)
	}

	attemptedItems.Add(int64(batchSize))

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		stats.Record(elapsed.Seconds()*1000, err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		stats.Record(elapsed.Seconds()*1000, fmt.Errorf("status %d", resp.StatusCode))
		return
	}

	if rej := sender.ParseRejected(respBody); rej > 0 {
		rejectedItems.Add(int64(rej))
	}
	stats.Record(elapsed.Seconds()*1000, nil)
}
