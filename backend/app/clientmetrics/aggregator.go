package clientmetrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// Sink is where a flush goes: the repositories in the self-hosted builds. It is the
// whole contract between the aggregator and storage, so an edge deployment can swap
// it for a call to the backend without the backend noticing.
type Sink interface {
	WriteHistograms(ctx context.Context, cells []shared.MetricHistogramCell) error
	WritePoints(ctx context.Context, points []models.MetricPoint) error
	WriteExemplars(ctx context.Context, exemplars []shared.MetricExemplar) error
}

const (
	aggregatorShards   = 64
	exemplarsPerHour   = 3
	maxCumulativeState = 50_000
	cumulativeIdle     = 15 * time.Minute
)

type cellKey struct {
	project uuid.UUID
	name    string
	minute  int64
	dims    string
}

type histogramCell struct {
	dims map[string]string
	hist *shared.Histogram
}

type counterCell struct {
	dims  map[string]string
	value float64
}

type exemplarKey struct {
	project uuid.UUID
	name    string
	dims    string
	hour    int64
}

type cumulativeState struct {
	hist *shared.Histogram
	seen time.Time
}

type aggregatorShard struct {
	mu         sync.Mutex
	histograms map[cellKey]*histogramCell
	counters   map[cellKey]*counterCell
	exemplars  []shared.MetricExemplar
	exemplarN  map[exemplarKey]int
	cumulative map[cellKey]*cumulativeState
}

// aggregator holds the cells between two flushes. Memory is bounded by maxCells, not
// by traffic: a million page views of one route on one release in one minute are one
// cell.
type aggregator struct {
	shards   [aggregatorShards]aggregatorShard
	cells    atomic.Int64
	maxCells int64
}

func newAggregator(maxCells int) *aggregator {
	a := &aggregator{maxCells: int64(maxCells)}
	for i := range a.shards {
		a.shards[i].reset()
		a.shards[i].exemplarN = make(map[exemplarKey]int)
		a.shards[i].cumulative = make(map[cellKey]*cumulativeState)
	}
	return a
}

func (s *aggregatorShard) reset() {
	s.histograms = make(map[cellKey]*histogramCell)
	s.counters = make(map[cellKey]*counterCell)
	s.exemplars = nil
}

func (a *aggregator) shard(key cellKey) *aggregatorShard {
	h := hashDims(key.dims) ^ uint64(key.minute)
	for _, b := range key.project[:8] {
		h = h*31 + uint64(b)
	}
	for i := 0; i < len(key.name); i++ {
		h = h*31 + uint64(key.name[i])
	}
	return &a.shards[h%aggregatorShards]
}

func newKey(projectId uuid.UUID, name string, at time.Time, dims map[string]string) cellKey {
	return cellKey{project: projectId, name: name, minute: at.Unix() / 60, dims: canonicalDims(dims)}
}

// reserve reports whether a new cell fits under the memory cap.
func (a *aggregator) reserve() bool {
	if a.cells.Add(1) > a.maxCells {
		a.cells.Add(-1)
		return false
	}
	return true
}

func (a *aggregator) addHistogram(projectId uuid.UUID, name string, at time.Time, dims map[string]string, h *shared.Histogram) bool {
	if h == nil || h.Count <= 0 {
		return true
	}
	key := newKey(projectId, name, at, dims)
	shard := a.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	cell := shard.histograms[key]
	if cell == nil {
		if !a.reserve() {
			return false
		}
		cell = &histogramCell{dims: dims, hist: shared.NewHistogram()}
		shard.histograms[key] = cell
	}
	cell.hist.Merge(h)
	return true
}

func (a *aggregator) observe(projectId uuid.UUID, name string, at time.Time, dims map[string]string, value, weight float64) bool {
	key := newKey(projectId, name, at, dims)
	shard := a.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	cell := shard.histograms[key]
	if cell == nil {
		if !a.reserve() {
			return false
		}
		cell = &histogramCell{dims: dims, hist: shared.NewHistogram()}
		shard.histograms[key] = cell
	}
	cell.hist.Observe(value, weight)
	return true
}

func (a *aggregator) count(projectId uuid.UUID, name string, at time.Time, dims map[string]string, n float64) bool {
	if !(n > 0) {
		return true
	}
	key := newKey(projectId, name, at, dims)
	shard := a.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	cell := shard.counters[key]
	if cell == nil {
		if !a.reserve() {
			return false
		}
		cell = &counterCell{dims: dims}
		shard.counters[key] = cell
	}
	cell.value += n
	return true
}

// exemplar keeps at most a few sessions per cell per hour: enough to open a replay
// from a bad percentile, never a list of everyone who had a bad time.
func (a *aggregator) exemplar(projectId uuid.UUID, name string, at time.Time, dims map[string]string, value float64, sessionId, target string) {
	canonical := canonicalDims(dims)
	key := exemplarKey{project: projectId, name: name, dims: canonical, hour: at.Unix() / 3600}
	shard := a.shard(cellKey{project: projectId, name: name, dims: canonical})
	shard.mu.Lock()
	defer shard.mu.Unlock()
	if shard.exemplarN[key] >= exemplarsPerHour {
		return
	}
	shard.exemplarN[key]++
	shard.exemplars = append(shard.exemplars, shared.MetricExemplar{ProjectId: projectId, Name: name, RecordedAt: at.UTC(), Dims: dims, Value: value, SessionId: sessionId, Target: target})
}

// delta turns one sample of a cumulative series into what was added since the
// previous sample. The first sample of a series only sets the baseline.
func (a *aggregator) delta(projectId uuid.UUID, name string, tags map[string]string, current *shared.Histogram, now time.Time) *shared.Histogram {
	key := cellKey{project: projectId, name: name, dims: canonicalDims(tags)}
	shard := a.shard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()
	previous := shard.cumulative[key]
	if previous == nil {
		if len(shard.cumulative) < maxCumulativeState/aggregatorShards {
			shard.cumulative[key] = &cumulativeState{hist: current, seen: now}
		}
		return nil
	}
	delta, ok := current.Sub(previous.hist)
	previous.hist, previous.seen = current, now
	if !ok {
		return current
	}
	return delta
}

type flushed struct {
	histograms []shared.MetricHistogramCell
	points     []models.MetricPoint
	exemplars  []shared.MetricExemplar
}

// drain hands over everything collected and starts new maps. Requests arriving
// during the write go into the new maps, so a slow storage never blocks ingest.
func (a *aggregator) drain(now time.Time) flushed {
	var out flushed
	hour := now.Unix() / 3600
	for i := range a.shards {
		shard := &a.shards[i]
		shard.mu.Lock()
		histograms, counters, exemplars := shard.histograms, shard.counters, shard.exemplars
		shard.reset()
		for key := range shard.exemplarN {
			if key.hour < hour-1 {
				delete(shard.exemplarN, key)
			}
		}
		for key, state := range shard.cumulative {
			if now.Sub(state.seen) > cumulativeIdle {
				delete(shard.cumulative, key)
			}
		}
		shard.mu.Unlock()

		for key, cell := range histograms {
			out.histograms = append(out.histograms, shared.MetricHistogramCell{
				ProjectId: key.project, Name: key.name, RecordedAt: time.Unix(key.minute*60, 0).UTC(), Dims: cell.dims, Hist: cell.hist,
			})
		}
		for key, cell := range counters {
			out.points = append(out.points, models.MetricPoint{
				ProjectId: key.project, Name: key.name, Value: cell.value, Tags: cell.dims, RecordedAt: time.Unix(key.minute*60, 0).UTC(),
			})
		}
		out.exemplars = append(out.exemplars, exemplars...)
		a.cells.Add(-int64(len(histograms) + len(counters)))
	}
	return out
}
