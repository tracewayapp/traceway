package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
)

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func buildBodyJS(url string, spans int) []byte {
	stack := fmt.Sprintf(`RangeError: cannot reserve 3 units for order ord_1042, only 2 in stock
    at file:///bench/%s:1:435
    at p (file:///bench/%s:1:554)
    at file:///bench/%s:1:1180
    at file:///bench/%s:1:1717`, url, url, url, url)
	return buildBody("nodejs", "RangeError", "bench", stack, spans)
}

func buildBodyDart(trace string, spans int) []byte {
	return buildBody("dart", "DartError", "bench", trace, spans)
}

func buildBody(lang, excType, excMsg, stack string, spans int) []byte {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "processor-bench")
	rs.Resource().Attributes().PutStr("telemetry.sdk.language", lang)
	ss := rs.ScopeSpans().AppendEmpty()
	for i := 0; i < spans; i++ {
		sp := ss.Spans().AppendEmpty()
		sp.SetName("POST /orders/fulfill")
		sp.SetKind(ptrace.SpanKindServer)
		var tid pcommon.TraceID
		var sid pcommon.SpanID
		copy(tid[:], fmt.Sprintf("%016d", i))
		copy(sid[:], fmt.Sprintf("%08d", i))
		sp.SetTraceID(tid)
		sp.SetSpanID(sid)
		ev := sp.Events().AppendEmpty()
		ev.SetName("exception")
		ev.Attributes().PutStr("exception.type", excType)
		ev.Attributes().PutStr("exception.message", excMsg)
		ev.Attributes().PutStr("exception.stacktrace", stack)
	}
	req := ptraceotlp.NewExportRequestFromTraces(td)
	data, err := req.MarshalProto()
	if err != nil {
		panic(err)
	}
	return data
}

type stepResult struct {
	Connections  int     `json:"connections"`
	DurationSec  float64 `json:"duration_sec"`
	Sent         int64   `json:"sent"`
	Ok           int64   `json:"ok"`
	Rejected     int64   `json:"rejected"`
	Errors       int64   `json:"errors"`
	OkReqPerSec  float64 `json:"ok_req_per_sec"`
	StacksPerSec float64 `json:"stacks_per_sec"`
	P50Ms        float64 `json:"p50_ms"`
	P99Ms        float64 `json:"p99_ms"`
}

func main() {
	target := flag.String("target", "http://localhost:4318/v1/traces", "")
	corpusFile := flag.String("corpus", "./corpus/corpus.json", "")
	connSteps := flag.String("connections", "4,8,16,32,64,128", "")
	stepDur := flag.Duration("step-duration", 30*time.Second, "")
	spansPerReq := flag.Int("spans-per-request", 20, "")
	outFile := flag.String("out", "loadgen-results.json", "")
	flag.Parse()

	raw, err := os.ReadFile(*corpusFile)
	if err != nil {
		panic(err)
	}
	var c struct {
		Language string   `json:"language"`
		Urls     []string `json:"urls"`
		Builds   []struct {
			BuildID string `json:"buildId"`
			Trace   string `json:"trace"`
		} `json:"builds"`
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		panic(err)
	}
	var bodies [][]byte
	if c.Language == "dart" {
		bodies = make([][]byte, len(c.Builds))
		for i, b := range c.Builds {
			bodies[i] = buildBodyDart(b.Trace, *spansPerReq)
		}
	} else {
		bodies = make([][]byte, len(c.Urls))
		for i, u := range c.Urls {
			bodies[i] = buildBodyJS(u, *spansPerReq)
		}
	}
	if len(bodies) == 0 {
		panic("corpus has no entries")
	}
	fmt.Printf("prepared %d %s bodies, %d spans each, %d bytes first\n", len(bodies), orDefault(c.Language, "js"), *spansPerReq, len(bodies[0]))

	var results []stepResult
	for _, s := range strings.Split(*connSteps, ",") {
		conns, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			panic(err)
		}
		tr := &http.Transport{MaxIdleConns: conns * 2, MaxIdleConnsPerHost: conns * 2}
		client := &http.Client{Transport: tr, Timeout: 30 * time.Second}
		var sent, ok, rejected, errs int64
		var bodyIdx int64
		latCh := make(chan float64, 65536)
		deadline := time.Now().Add(*stepDur)
		var wg sync.WaitGroup
		for w := 0; w < conns; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for time.Now().Before(deadline) {
					idx := atomic.AddInt64(&bodyIdx, 1)
					body := bodies[idx%int64(len(bodies))]
					atomic.AddInt64(&sent, 1)
					t0 := time.Now()
					resp, err := client.Post(*target, "application/x-protobuf", bytes.NewReader(body))
					lat := float64(time.Since(t0).Microseconds()) / 1000.0
					if err != nil {
						atomic.AddInt64(&errs, 1)
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						atomic.AddInt64(&ok, 1)
						select {
						case latCh <- lat:
						default:
						}
					} else {
						atomic.AddInt64(&rejected, 1)
					}
				}
			}()
		}
		wg.Wait()
		close(latCh)
		var lats []float64
		for l := range latCh {
			lats = append(lats, l)
		}
		sort.Float64s(lats)
		pct := func(p float64) float64 {
			if len(lats) == 0 {
				return 0
			}
			return lats[int(float64(len(lats)-1)*p)]
		}
		r := stepResult{
			Connections:  conns,
			DurationSec:  stepDur.Seconds(),
			Sent:         sent,
			Ok:           ok,
			Rejected:     rejected,
			Errors:       errs,
			OkReqPerSec:  float64(ok) / stepDur.Seconds(),
			StacksPerSec: float64(ok*int64(*spansPerReq)) / stepDur.Seconds(),
			P50Ms:        pct(0.50),
			P99Ms:        pct(0.99),
		}
		results = append(results, r)
		line, _ := json.Marshal(r)
		fmt.Println(string(line))
		tr.CloseIdleConnections()
	}
	data, _ := json.MarshalIndent(results, "", "  ")
	if err := os.MkdirAll(filepath.Dir(*outFile), 0o755); err != nil && filepath.Dir(*outFile) != "." {
		panic(err)
	}
	if err := os.WriteFile(*outFile, data, 0o644); err != nil {
		panic(err)
	}
}
