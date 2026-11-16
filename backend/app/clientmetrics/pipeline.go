package clientmetrics

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// Reasons an observation is dropped. They are the `reason` tag of
// traceway.client_metrics.dropped and the keys of the health block.
const (
	ReasonUnknownMetric = "unknown_metric"
	ReasonCardinality   = "cardinality"
	ReasonLate          = "late"
	ReasonRateLimit     = "rate_limit"
	ReasonRequestLimit  = "request_limit"
	ReasonMemory        = "memory"
	ReasonUnsupported   = "unsupported"
	ReasonTemporality   = "temporality"
	ReasonFlushFailed   = "flush_failed"
	// ReasonDuplicate is a resend of a payload that had arrived the first time.
	ReasonDuplicate = "duplicate"
)

var reasons = []string{ReasonUnknownMetric, ReasonCardinality, ReasonLate, ReasonRateLimit, ReasonRequestLimit, ReasonMemory, ReasonUnsupported, ReasonTemporality, ReasonFlushFailed, ReasonDuplicate}

const (
	// MaxObservationsPerRequest is the number of data points one request may carry.
	MaxObservationsPerRequest = 2000
	// MaxBodyBytes caps a client metrics request, raw and decompressed. A payload
	// of a whole session is a few KB.
	MaxBodyBytes = 64 << 10

	maxSampleWeight = 1000
	maxPast         = 72 * time.Hour
	maxFuture       = 5 * time.Minute

	attrSampleRate = "traceway.sample_rate"
	attrSentAt     = "traceway.sent_at"
	// AttrAggregated marks a resource whose SDK sends its performance numbers as
	// aggregates, so the events it also uploads are not counted a second time.
	AttrAggregated  = "traceway.metrics"
	AggregatedValue = "aggregated"
	attrSessionId   = "session.id"
	attrBatchId     = "traceway.batch_id"
	attrRetry       = "traceway.retry"
	attrTarget      = "traceway.target"

	maxBatchIdLength = 64
	maxTargetLength  = 100
	stateOverlap     = 2 * time.Minute
	seriesRefresh    = time.Minute
	maxUnsaved       = 20_000
)

// Client is what the server knows about the sender of a request. None of it is
// stored except the country.
type Client struct {
	IP string
	// Country is an ISO 3166-1 alpha-2 code, or empty when it is not known.
	Country string
}

// StateStore shares what the guard admitted between instances and across restarts.
type StateStore interface {
	SaveGuardEntries(ctx context.Context, entries []GuardEntry) error
	// LoadGuardEntries returns the entries of the day and the registrations, created
	// after since.
	LoadGuardEntries(ctx context.Context, day int64, since time.Time) ([]GuardEntry, error)
	PruneGuardEntries(ctx context.Context, beforeDay int64) error
}

type Config struct {
	FlushInterval time.Duration
	// MaxCells bounds the cells held between two flushes, and so the memory.
	MaxCells int
	// SeriesBudget is the number of series one metric may have per project per day.
	SeriesBudget int
	// IPObservationsPerMinute is what one address may add to one project per
	// minute, so a single client cannot move a percentile.
	IPObservationsPerMinute int
	// DedupMB is the memory of the filter that recognizes resent payloads.
	DedupMB int
	// Instances is the number of backend instances sharing the traffic; the per-address
	// cap is divided by it. Nil means one.
	Instances func() int
	// Store is optional: without it the caps are per process and reset on restart.
	Store StateStore
	// HotSeries returns the stored series of metrics that have at least minSeries of
	// them since from. Optional.
	HotSeries func(ctx context.Context, from time.Time, minSeries, limit int) ([]SeriesSet, error)
	OnError   func(error)
	// OnMetrics is told which metrics a flush wrote, for the metric registry.
	OnMetrics func(ctx context.Context, projectId uuid.UUID, metrics []SeenMetric)
}

// SeenMetric is a metric a flush wrote for a project.
type SeenMetric struct {
	Name      string
	Unit      string
	Histogram bool
}

func (c Config) withDefaults() Config {
	if c.FlushInterval <= 0 {
		c.FlushInterval = 10 * time.Second
	}
	if c.MaxCells <= 0 {
		c.MaxCells = 500_000
	}
	if c.IPObservationsPerMinute <= 0 {
		c.IPObservationsPerMinute = 6000
	}
	return c
}

type Pipeline struct {
	cfg     Config
	sink    Sink
	guard   *guard
	agg     *aggregator
	limiter *ipLimiter
	batches *batchFilter

	stateMu      sync.Mutex
	unsaved      []GuardEntry
	stateLoaded  time.Time
	seriesLoaded time.Time
	stateDay     int64

	requests     atomic.Int64
	observations atomic.Int64
	folded       atomic.Int64
	flushes      atomic.Int64
	lastFlushMs  atomic.Int64
	lastFlushed  atomic.Int64
	duplicates   atomic.Int64
	dropped      [10]atomic.Int64

	flushMu sync.Mutex
	started atomic.Bool
}

func New(sink Sink, cfg Config) *Pipeline {
	cfg = cfg.withDefaults()
	return &Pipeline{
		cfg:     cfg,
		sink:    sink,
		guard:   newGuard(cfg.SeriesBudget),
		agg:     newAggregator(cfg.MaxCells),
		limiter: newIPLimiter(cfg.IPObservationsPerMinute, cfg.Instances),
		batches: newBatchFilter(cfg.DedupMB, time.Now()),
	}
}

func (p *Pipeline) drop(reason string, n int) {
	for i, r := range reasons {
		if r == reason {
			p.dropped[i].Add(int64(n))
			return
		}
	}
}

// Start runs the flush loop until ctx is done. The server has no graceful shutdown
// of its own, so the binary calls Flush on SIGTERM; a crash loses at most one flush
// interval, which is accepted for statistics.
func (p *Pipeline) Start(ctx context.Context) {
	if !p.started.CompareAndSwap(false, true) {
		return
	}
	p.SyncState(ctx, time.Now())
	go func() {
		ticker := time.NewTicker(p.cfg.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = p.Flush(context.WithoutCancel(ctx))
				return
			case <-ticker.C:
				_ = p.Flush(ctx)
				p.SyncState(ctx, time.Now())
			}
		}
	}()
}

// Flush writes what was collected: one batch per table, however many requests fed it.
func (p *Pipeline) Flush(ctx context.Context) error {
	p.flushMu.Lock()
	defer p.flushMu.Unlock()
	started := time.Now()
	out := p.agg.drain(started)
	p.limiter.prune(started)
	if len(out.histograms)+len(out.points)+len(out.exemplars) == 0 {
		return nil
	}
	var firstErr error
	fail := func(err error, lost int) {
		if err == nil {
			return
		}
		p.drop(ReasonFlushFailed, lost)
		if firstErr == nil {
			firstErr = err
		}
		if p.cfg.OnError != nil {
			p.cfg.OnError(err)
		}
	}
	fail(p.sink.WriteHistograms(ctx, out.histograms), len(out.histograms))
	fail(p.sink.WritePoints(ctx, out.points), len(out.points))
	fail(p.sink.WriteExemplars(ctx, out.exemplars), 0)
	p.flushes.Add(1)
	p.lastFlushed.Store(int64(len(out.histograms) + len(out.points)))
	p.lastFlushMs.Store(time.Since(started).Milliseconds())
	if p.cfg.OnMetrics != nil {
		for projectId, metrics := range seenMetrics(out) {
			p.cfg.OnMetrics(ctx, projectId, metrics)
		}
	}
	return firstErr
}

// SyncState writes what the guard admitted since the last call and reads what other
// instances admitted, so a cap is one number for the deployment. Between two calls
// each instance can admit on its own, which is the only overshoot.
func (p *Pipeline) SyncState(ctx context.Context, now time.Time) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	report := func(err error) {
		if err != nil && p.cfg.OnError != nil {
			p.cfg.OnError(err)
		}
	}
	day := now.Unix() / secondsPerDay
	if store := p.cfg.Store; store != nil {
		p.unsaved = append(p.unsaved, p.guard.takePending()...)
		if len(p.unsaved) > 0 {
			if err := store.SaveGuardEntries(ctx, p.unsaved); err != nil {
				report(err)
				if len(p.unsaved) > maxUnsaved {
					p.unsaved = p.unsaved[len(p.unsaved)-maxUnsaved:]
				}
			} else {
				p.unsaved = nil
			}
		}
		since := time.Time{}
		if p.stateDay == day && !p.stateLoaded.IsZero() {
			since = p.stateLoaded.Add(-stateOverlap)
		}
		entries, err := store.LoadGuardEntries(ctx, day, since)
		if err != nil {
			report(err)
		} else {
			p.guard.merge(entries, now)
			p.stateLoaded = now
			if p.stateDay != day {
				if p.stateDay != 0 {
					report(store.PruneGuardEntries(ctx, day-1))
				}
				p.stateDay = day
			}
		}
	} else {
		p.guard.takePending()
	}
	if p.cfg.HotSeries != nil && now.Sub(p.seriesLoaded) >= seriesRefresh {
		p.seriesLoaded = now
		sets, err := p.cfg.HotSeries(ctx, time.Unix(day*secondsPerDay, 0).UTC(), p.guard.seriesBudget/2, 2*p.guard.seriesBudget)
		if err != nil {
			report(err)
		} else {
			p.guard.mergeSeries(sets, now)
		}
	}
}

func seenMetrics(out flushed) map[uuid.UUID][]SeenMetric {
	type key struct {
		project uuid.UUID
		name    string
	}
	seen := make(map[key]struct{})
	result := make(map[uuid.UUID][]SeenMetric)
	add := func(projectId uuid.UUID, name string, histogram bool) {
		k := key{projectId, name}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		metric := SeenMetric{Name: name, Histogram: histogram}
		if entry, ok := Lookup(name); ok {
			metric.Unit = entry.Unit
		}
		if !histogram && metric.Unit == "" {
			metric.Unit = "count"
		}
		result[projectId] = append(result[projectId], metric)
	}
	for _, cell := range out.histograms {
		add(cell.ProjectId, cell.Name, true)
	}
	for _, point := range out.points {
		add(point.ProjectId, point.Name, false)
	}
	return result
}

// IngestResult says what one request contributed.
type IngestResult struct {
	Accepted int
	Dropped  int
}

func attrMap(kvs []*commonpb.KeyValue) map[string]string {
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		switch v := kv.GetValue().GetValue().(type) {
		case *commonpb.AnyValue_StringValue:
			out[kv.GetKey()] = v.StringValue
		case *commonpb.AnyValue_BoolValue:
			out[kv.GetKey()] = strconv.FormatBool(v.BoolValue)
		case *commonpb.AnyValue_IntValue:
			out[kv.GetKey()] = strconv.FormatInt(v.IntValue, 10)
		case *commonpb.AnyValue_DoubleValue:
			out[kv.GetKey()] = strconv.FormatFloat(v.DoubleValue, 'f', -1, 64)
		}
	}
	return out
}

// resourceContext is what a payload says about itself as a whole.
type resourceContext struct {
	dims   map[string]string
	weight float64
	offset time.Duration
	client Client
}

func countDataPoints(rm *metricspb.ResourceMetrics) int {
	n := 0
	for _, sm := range rm.GetScopeMetrics() {
		for _, m := range sm.GetMetrics() {
			switch data := m.GetData().(type) {
			case *metricspb.Metric_ExponentialHistogram:
				n += len(data.ExponentialHistogram.GetDataPoints())
			case *metricspb.Metric_Histogram:
				n += len(data.Histogram.GetDataPoints())
			case *metricspb.Metric_Sum:
				n += len(data.Sum.GetDataPoints())
			case *metricspb.Metric_Gauge:
				n += len(data.Gauge.GetDataPoints())
			}
		}
	}
	return n
}

func newResourceContext(attrs map[string]string, client Client, now time.Time) resourceContext {
	ctx := resourceContext{dims: attrs, weight: 1, client: client}
	if name := attrs["service.name"]; name != "" && attrs["server_name"] != name {
		// A copy: the caller's map may be a stored record's resource attributes.
		ctx.dims = make(map[string]string, len(attrs)+1)
		for key, value := range attrs {
			ctx.dims[key] = value
		}
		ctx.dims["server_name"] = name
	}
	if rate, err := strconv.ParseFloat(attrs[attrSampleRate], 64); err == nil && rate > 0 && rate < 1 {
		ctx.weight = 1 / rate
		if ctx.weight > maxSampleWeight {
			ctx.weight = maxSampleWeight
		}
	}
	// A device clock can be hours off. The payload says when the device sent it, by
	// its own clock, so the difference to the receive time corrects every sample.
	if sentMs, err := strconv.ParseFloat(attrs[attrSentAt], 64); err == nil && sentMs > 0 {
		ctx.offset = now.Sub(time.UnixMilli(int64(sentMs)))
	}
	return ctx
}

// sampleTime places a sample on the server's clock, or reports that it is too old or
// too far ahead to keep.
func (rc resourceContext) sampleTime(unixNano uint64, now time.Time) (time.Time, bool) {
	if unixNano == 0 {
		return now, true
	}
	at := time.Unix(0, int64(unixNano)).Add(rc.offset)
	if now.Sub(at) > maxPast || at.Sub(now) > maxFuture {
		return time.Time{}, false
	}
	return at, true
}

// IngestOTLP adds one OTLP metrics request of a client project. Nothing is written:
// the request is done once its samples are in memory.
func (p *Pipeline) IngestOTLP(projectId uuid.UUID, req *colmetricspb.ExportMetricsServiceRequest, client Client, now time.Time) IngestResult {
	p.requests.Add(1)
	var result IngestResult
	points := 0
	refuse := func(reason string, n int) {
		p.drop(reason, n)
		result.Dropped += n
	}
	for _, rm := range req.GetResourceMetrics() {
		resource := attrMap(rm.GetResource().GetAttributes())
		rc := newResourceContext(resource, client, now)
		if id := resource[attrBatchId]; id != "" && len(id) <= maxBatchIdLength {
			// Every payload is recorded, and only a resend is checked: a first upload is
			// never refused because the filter happens to know its bits.
			if p.batches.seen(projectId, id, now) && resource[attrRetry] == "true" {
				p.duplicates.Add(1)
				refuse(ReasonDuplicate, countDataPoints(rm))
				continue
			}
		}
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				name := m.GetName()
				entry, known := Lookup(name)
				switch data := m.GetData().(type) {
				case *metricspb.Metric_ExponentialHistogram:
					if data.ExponentialHistogram.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA {
						refuse(ReasonTemporality, len(data.ExponentialHistogram.GetDataPoints()))
						continue
					}
					for _, dp := range data.ExponentialHistogram.GetDataPoints() {
						if points++; points > MaxObservationsPerRequest {
							refuse(ReasonRequestLimit, 1)
							continue
						}
						if known && entry.Shape != Distribution {
							refuse(ReasonUnsupported, 1)
							continue
						}
						h := shared.HistogramFromExponential(shared.ExponentialHistogramSource{
							Scale: dp.GetScale(), Sum: dp.GetSum(), HasSum: dp.Sum != nil,
							Min: dp.GetMin(), Max: dp.GetMax(), HasMinMax: dp.Min != nil && dp.Max != nil,
							ZeroCount: float64(dp.GetZeroCount()), Offset: dp.GetPositive().GetOffset(), BucketCnts: dp.GetPositive().GetBucketCounts(),
						}, rc.weight)
						reason := p.addHistogram(projectId, name, entry, rc, attrMap(dp.GetAttributes()), dp.GetTimeUnixNano(), h, dp.GetExemplars(), now)
						if reason != "" {
							refuse(reason, 1)
							continue
						}
						result.Accepted++
					}
				case *metricspb.Metric_Histogram:
					if data.Histogram.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA {
						refuse(ReasonTemporality, len(data.Histogram.GetDataPoints()))
						continue
					}
					for _, dp := range data.Histogram.GetDataPoints() {
						if points++; points > MaxObservationsPerRequest {
							refuse(ReasonRequestLimit, 1)
							continue
						}
						if known && entry.Shape != Distribution {
							refuse(ReasonUnsupported, 1)
							continue
						}
						h := shared.HistogramFromExplicit(dp.GetExplicitBounds(), dp.GetBucketCounts(), dp.GetSum(), dp.Sum != nil,
							dp.GetMin(), dp.GetMax(), dp.Min != nil && dp.Max != nil, rc.weight)
						reason := p.addHistogram(projectId, name, entry, rc, attrMap(dp.GetAttributes()), dp.GetTimeUnixNano(), h, dp.GetExemplars(), now)
						if reason != "" {
							refuse(reason, 1)
							continue
						}
						result.Accepted++
					}
				case *metricspb.Metric_Sum:
					// A cumulative counter needs an identity per client to be differenced,
					// which is exactly what client metrics do not keep.
					if data.Sum.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA || !data.Sum.GetIsMonotonic() {
						refuse(ReasonTemporality, len(data.Sum.GetDataPoints()))
						continue
					}
					for _, dp := range data.Sum.GetDataPoints() {
						if points++; points > MaxObservationsPerRequest {
							refuse(ReasonRequestLimit, 1)
							continue
						}
						if known && entry.Shape != Counter {
							refuse(ReasonUnsupported, 1)
							continue
						}
						value := dp.GetAsDouble()
						if _, isInt := dp.GetValue().(*metricspb.NumberDataPoint_AsInt); isInt {
							value = float64(dp.GetAsInt())
						}
						reason := p.addCount(projectId, name, entry, rc, attrMap(dp.GetAttributes()), dp.GetTimeUnixNano(), value, now)
						if reason != "" {
							refuse(reason, 1)
							continue
						}
						result.Accepted++
					}
				case *metricspb.Metric_Gauge:
					// A wall-clock sample of one device says nothing about a population.
					refuse(ReasonUnsupported, len(data.Gauge.GetDataPoints()))
				default:
					refuse(ReasonUnsupported, 1)
				}
			}
		}
	}
	p.observations.Add(int64(result.Accepted))
	return result
}

func (p *Pipeline) addHistogram(projectId uuid.UUID, name string, entry *Metric, rc resourceContext, attrs map[string]string, unixNano uint64, h *shared.Histogram, exemplars []*metricspb.Exemplar, now time.Time) string {
	if h.Count <= 0 {
		return ""
	}
	at, ok := rc.sampleTime(unixNano, now)
	if !ok {
		return ReasonLate
	}
	if !p.limiter.allow(projectId, rc.client.IP, h.Count/rc.weight, now) {
		return ReasonRateLimit
	}
	resolved := p.guard.resolve(projectId, name, entry, rc.dims, attrs, rc.client.Country, now)
	if resolved.reason != "" {
		return resolved.reason
	}
	p.folded.Add(int64(resolved.folded))
	limit := float64(customValueMax)
	if entry != nil && entry.Max > 0 {
		limit = entry.Max
	}
	clampHistogram(h, limit)
	if !p.agg.addHistogram(projectId, name, at, resolved.dims, h) {
		return ReasonMemory
	}
	for _, exemplar := range exemplars {
		filtered := attrMap(exemplar.GetFilteredAttributes())
		sessionId := filtered[attrSessionId]
		if _, err := uuid.Parse(sessionId); err != nil {
			continue
		}
		target := filtered[attrTarget]
		if len(target) > maxTargetLength {
			target = target[:maxTargetLength]
		}
		value := exemplar.GetAsDouble()
		if value > limit {
			value = limit
		}
		p.agg.exemplar(projectId, name, at, resolved.dims, value, sessionId, target)
	}
	return ""
}

func (p *Pipeline) addCount(projectId uuid.UUID, name string, entry *Metric, rc resourceContext, attrs map[string]string, unixNano uint64, value float64, now time.Time) string {
	if !(value > 0) {
		return ""
	}
	at, ok := rc.sampleTime(unixNano, now)
	if !ok {
		return ReasonLate
	}
	if value > MaxObservationsPerRequest*1000 {
		value = MaxObservationsPerRequest * 1000
	}
	if !p.limiter.allow(projectId, rc.client.IP, 1, now) {
		return ReasonRateLimit
	}
	resolved := p.guard.resolve(projectId, name, entry, rc.dims, attrs, rc.client.Country, now)
	if resolved.reason != "" {
		return resolved.reason
	}
	p.folded.Add(int64(resolved.folded))
	if !p.agg.count(projectId, name, at, resolved.dims, value*rc.weight) {
		return ReasonMemory
	}
	return ""
}

// ObserveEvent adds one observation of a catalog distribution: the path of SDKs that
// only send performance events. resource holds the record's resource attributes.
func (p *Pipeline) ObserveEvent(projectId uuid.UUID, name string, value float64, at time.Time, resource, attrs map[string]string, client Client, now time.Time) bool {
	entry, known := Lookup(name)
	if !known || entry.Shape != Distribution {
		p.drop(ReasonUnknownMetric, 1)
		return false
	}
	if value < 0 || value != value {
		p.drop(ReasonUnsupported, 1)
		return false
	}
	if now.Sub(at) > maxPast || at.Sub(now) > maxFuture {
		p.drop(ReasonLate, 1)
		return false
	}
	if !p.limiter.allow(projectId, client.IP, 1, now) {
		p.drop(ReasonRateLimit, 1)
		return false
	}
	rc := newResourceContext(resource, client, now)
	resolved := p.guard.resolve(projectId, name, entry, rc.dims, attrs, rc.client.Country, now)
	if resolved.reason != "" {
		p.drop(resolved.reason, 1)
		return false
	}
	p.folded.Add(int64(resolved.folded))
	if value > entry.Max {
		value = entry.Max
	}
	if !p.agg.observe(projectId, name, at, resolved.dims, value, 1) {
		p.drop(ReasonMemory, 1)
		return false
	}
	p.observations.Add(1)
	return true
}

// AddSeriesHistogram adds a histogram of a backend project, whose token is private
// and whose tags are its own business: no catalog, no caps. A cumulative series is
// differenced first.
func (p *Pipeline) AddSeriesHistogram(projectId uuid.UUID, name string, at time.Time, tags map[string]string, h *shared.Histogram, cumulative bool, now time.Time) {
	if cumulative {
		if h = p.agg.delta(projectId, name, tags, h, now); h == nil {
			return
		}
	}
	if !p.agg.addHistogram(projectId, name, at, tags, h) {
		p.drop(ReasonMemory, 1)
		return
	}
	p.observations.Add(1)
}

// clampHistogram moves everything above limit into the bucket of limit.
func clampHistogram(h *shared.Histogram, limit float64) {
	if h.Max <= limit {
		return
	}
	top := shared.HistogramBucketIndex(limit)
	var moved float64
	for index, count := range h.Buckets {
		if index > top {
			moved += count
			delete(h.Buckets, index)
		}
	}
	if moved > 0 {
		h.Buckets[top] += moved
	}
	h.Max = limit
	if h.Min > limit {
		h.Min = limit
	}
	if mean := h.Sum / h.Count; mean > limit {
		h.Sum = limit * h.Count
	}
}

type ipKey struct {
	project uuid.UUID
	ip      string
}

type ipWindow struct {
	minute int64
	count  float64
}

type ipLimiter struct {
	limit     float64
	instances func() int
	mu        sync.Mutex
	seen      map[ipKey]*ipWindow
}

func newIPLimiter(limit int, instances func() int) *ipLimiter {
	return &ipLimiter{limit: float64(limit), instances: instances, seen: make(map[ipKey]*ipWindow)}
}

// share is this instance's part of the cap: a balancer spreads one address over every
// instance, so each may let through only its fraction.
func (l *ipLimiter) share() float64 {
	if l.instances == nil {
		return l.limit
	}
	if n := l.instances(); n > 1 {
		return l.limit / float64(n)
	}
	return l.limit
}

func (l *ipLimiter) allow(projectId uuid.UUID, ip string, n float64, now time.Time) bool {
	if ip == "" {
		return true
	}
	minute := now.Unix() / 60
	key := ipKey{projectId, ip}
	limit := l.share()
	l.mu.Lock()
	defer l.mu.Unlock()
	window := l.seen[key]
	if window == nil || window.minute != minute {
		window = &ipWindow{minute: minute}
		l.seen[key] = window
	}
	if window.count+n > limit {
		return false
	}
	window.count += n
	return true
}

func (l *ipLimiter) prune(now time.Time) {
	minute := now.Unix() / 60
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, window := range l.seen {
		if window.minute < minute {
			delete(l.seen, key)
		}
	}
}

// HealthStats is the clientMetrics block of /api/health/deep. Counters are since
// process start.
type HealthStats struct {
	Requests          int64            `json:"requests"`
	Observations      int64            `json:"observations"`
	Cells             int64            `json:"cells"`
	MaxCells          int64            `json:"maxCells"`
	FoldedDimensions  int64            `json:"foldedDimensions"`
	Flushes           int64            `json:"flushes"`
	LastFlushMs       int64            `json:"lastFlushMs"`
	LastFlushRows     int64            `json:"lastFlushRows"`
	DuplicatePayloads int64            `json:"duplicatePayloads"`
	Dropped           map[string]int64 `json:"dropped"`
	DroppedTotal      int64            `json:"droppedTotal"`
}

func (p *Pipeline) Snapshot() HealthStats {
	stats := HealthStats{
		Requests:          p.requests.Load(),
		Observations:      p.observations.Load(),
		Cells:             p.agg.cells.Load(),
		MaxCells:          p.agg.maxCells,
		FoldedDimensions:  p.folded.Load(),
		Flushes:           p.flushes.Load(),
		LastFlushMs:       p.lastFlushMs.Load(),
		LastFlushRows:     p.lastFlushed.Load(),
		DuplicatePayloads: p.duplicates.Load(),
		Dropped:           make(map[string]int64, len(reasons)),
	}
	for i, reason := range reasons {
		n := p.dropped[i].Load()
		stats.Dropped[reason] = n
		stats.DroppedTotal += n
	}
	return stats
}
