package main

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tracewayapp/traceway/backend/app/monitoring"
	"github.com/tracewayapp/traceway/backend/app/services"

	"github.com/google/uuid"
)

const latencyReservoirSize = 200000

type runConfig struct {
	corpusDir   string
	projectId   string
	label       string
	mode        string
	entries     int
	hot         int
	coldRatio   float64
	duration    time.Duration
	warmup      time.Duration
	concurrency int
	memCacheMB  int
	openEntries int
	openMB      int
	diskDir     string
	diskMB      int
	tier        string
	out         string
}

type runResult struct {
	Status        string  `json:"status"`
	Tier          string  `json:"tier"`
	Label         string  `json:"label"`
	Mode          string  `json:"mode"`
	Entries       int     `json:"entries"`
	Hot           int     `json:"hot"`
	ColdRatio     float64 `json:"coldRatio"`
	CorpusGB      float64 `json:"corpusGB"`
	DurationSec   float64 `json:"durationSec"`
	Concurrency   int     `json:"concurrency"`
	Resolves      int64   `json:"resolves"`
	RPS           float64 `json:"rps"`
	P50Us         float64 `json:"p50Us"`
	P95Us         float64 `json:"p95Us"`
	P99Us         float64 `json:"p99Us"`
	MaxUs         float64 `json:"maxUs"`
	UnresolvedFrm int64   `json:"unresolvedFrames"`
	RSSPeakMB     float64 `json:"rssPeakMB"`
	HeapAllocMB   float64 `json:"heapAllocMB"`
	NumGC         uint32  `json:"numGC"`
	GCPauseMs     float64 `json:"gcPauseMs"`
	CacheHits     uint64  `json:"cacheHits"`
	CacheMisses   uint64  `json:"cacheMisses"`
	CacheEvict    uint64  `json:"cacheEvictions"`
	DiskHits      uint64  `json:"diskHits"`
	StoreHits     uint64  `json:"storeHits"`
	Builds        uint64  `json:"builds"`
	DiskEvict     uint64  `json:"diskEvictions"`
	DiskBytesMB   float64 `json:"diskBytesMB"`
	ExitCode      int     `json:"exitCode,omitempty"`
}

type latencyReservoir struct {
	samples []int64
	seen    int64
	rng     *rand.Rand
}

func (r *latencyReservoir) record(ns int64) {
	r.seen++
	if len(r.samples) < latencyReservoirSize {
		r.samples = append(r.samples, ns)
		return
	}
	if i := r.rng.Int63n(r.seen); i < latencyReservoirSize {
		r.samples[i] = ns
	}
}

func runBench(cfg runConfig) error {
	manifest, err := loadManifest(cfg.corpusDir)
	if err != nil {
		return fmt.Errorf("loading corpus manifest (run generate first): %w", err)
	}
	if cfg.entries > manifest.Entries {
		return fmt.Errorf("requested %d entries but corpus has %d", cfg.entries, manifest.Entries)
	}

	switch cfg.mode {
	case "memory":
		maxBytes := int64(cfg.memCacheMB) << 20
		if cfg.memCacheMB == 0 {
			maxBytes = 1 << 60
		}
		services.InitSourceMapCache(cfg.entries*2+cfg.hot, maxBytes)
	case "disk":
		services.InitSourceMapCache(cfg.openEntries, int64(cfg.openMB)<<20)
		if err := os.RemoveAll(cfg.diskDir); err != nil {
			return err
		}
		if err := services.EnableSymbolicatorDiskCache(cfg.diskDir, int64(cfg.diskMB)<<20); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown mode %q (memory or disk)", cfg.mode)
	}

	projectId, err := uuid.Parse(cfg.projectId)
	if err != nil {
		return err
	}

	hotSet := make([]int, cfg.hot)
	for i := range hotSet {
		hotSet[i] = i * cfg.entries / cfg.hot
	}
	fileNames := make([]string, cfg.entries)
	for i := range fileNames {
		fileNames[i] = fmt.Sprintf(bundleFileFmt, i)
	}

	var rssPeak atomic.Uint64
	rssCtx, rssCancel := context.WithCancel(context.Background())
	defer rssCancel()
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-rssCtx.Done():
				return
			case <-ticker.C:
				if rss, ok := monitoring.ReadRSSBytes(); ok && rss > rssPeak.Load() {
					rssPeak.Store(rss)
				}
			}
		}
	}()

	var measuring atomic.Bool
	var resolves atomic.Int64
	var unresolved atomic.Int64
	reservoirs := make([]*latencyReservoir, cfg.concurrency)

	workCtx, workCancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for w := range cfg.concurrency {
		reservoirs[w] = &latencyReservoir{rng: rand.New(rand.NewSource(int64(w) + 42))}
		wg.Add(1)
		go func(res *latencyReservoir, seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for workCtx.Err() == nil {
				var idx int
				if cfg.coldRatio > 0 && rng.Float64() < cfg.coldRatio {
					idx = rng.Intn(cfg.entries)
				} else {
					idx = hotSet[rng.Intn(len(hotSet))]
				}
				trace := buildTrace(fileNames[idx], manifest.BundleBytes, rng)
				start := time.Now()
				out := services.ResolveStackTrace(workCtx, projectId, trace, nil)
				elapsed := time.Since(start)
				if measuring.Load() {
					res.record(elapsed.Nanoseconds())
					resolves.Add(1)
					if n := int64(3 - strings.Count(out, "src/module-")); n > 0 {
						unresolved.Add(n)
					}
				}
			}
		}(reservoirs[w], int64(w)+1000)
	}

	time.Sleep(cfg.warmup)
	measuring.Store(true)
	measureStart := time.Now()
	time.Sleep(cfg.duration)
	measuring.Store(false)
	measuredFor := time.Since(measureStart)
	workCancel()
	wg.Wait()
	rssCancel()

	type weightedSample struct {
		ns int64
		w  float64
	}
	var all []weightedSample
	var totalWeight float64
	for _, r := range reservoirs {
		if len(r.samples) == 0 {
			continue
		}
		w := float64(r.seen) / float64(len(r.samples))
		for _, ns := range r.samples {
			all = append(all, weightedSample{ns: ns, w: w})
		}
		totalWeight += float64(r.seen)
	}
	slices.SortFunc(all, func(a, b weightedSample) int {
		return cmp.Compare(a.ns, b.ns)
	})
	maxUs := 0.0
	if len(all) > 0 {
		maxUs = float64(all[len(all)-1].ns) / 1e3
	}
	percentileUs := func(p float64) float64 {
		if len(all) == 0 {
			return 0
		}
		threshold := p * totalWeight
		cum := 0.0
		for _, s := range all {
			cum += s.w
			if cum >= threshold {
				return float64(s.ns) / 1e3
			}
		}
		return maxUs
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	smStats := services.SourceMapStats()

	result := runResult{
		Status:        "ok",
		Tier:          cfg.tier,
		Label:         cfg.label,
		Mode:          cfg.mode,
		Entries:       cfg.entries,
		Hot:           cfg.hot,
		ColdRatio:     cfg.coldRatio,
		CorpusGB:      float64(cfg.entries) * float64(manifest.ResolverBytes) / (1 << 30),
		DurationSec:   measuredFor.Seconds(),
		Concurrency:   cfg.concurrency,
		Resolves:      resolves.Load(),
		RPS:           float64(resolves.Load()) / measuredFor.Seconds(),
		P50Us:         percentileUs(0.50),
		P95Us:         percentileUs(0.95),
		P99Us:         percentileUs(0.99),
		MaxUs:         maxUs,
		UnresolvedFrm: unresolved.Load(),
		RSSPeakMB:     float64(rssPeak.Load()) / (1 << 20),
		HeapAllocMB:   float64(memStats.HeapAlloc) / (1 << 20),
		NumGC:         memStats.NumGC,
		GCPauseMs:     float64(memStats.PauseTotalNs) / 1e6,
		CacheHits:     smStats.Hits,
		CacheMisses:   smStats.Misses,
		CacheEvict:    smStats.Evictions,
		DiskHits:      smStats.DiskHits,
		StoreHits:     smStats.StoreHits,
		Builds:        smStats.Builds,
		DiskEvict:     smStats.DiskEvictions,
		DiskBytesMB:   float64(smStats.DiskBytes) / (1 << 20),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if cfg.out == "" || cfg.out == "-" {
		_, err = os.Stdout.Write(append(data, '\n'))
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cfg.out, append(data, '\n'), 0o644)
}

func buildTrace(file string, bundleLen int, rng *rand.Rand) string {
	var sb strings.Builder
	sb.Grow(64 + 3*(len(file)+32))
	sb.WriteString("Error: bench\n")
	for range 3 {
		sb.WriteString("anonymous()\n    ")
		sb.WriteString(file)
		sb.WriteString(":1:")
		sb.WriteString(strconv.Itoa(1 + rng.Intn(bundleLen)))
		sb.WriteByte('\n')
	}
	return sb.String()
}
