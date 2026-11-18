package shared

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

// MetricHistogramCell is the stored unit of a distribution metric: every observation
// of one metric with one set of dimensions in one minute, merged. Its size depends on
// the number of dimension combinations, never on the number of clients.
type MetricHistogramCell struct {
	ProjectId  uuid.UUID
	Name       string
	RecordedAt time.Time
	Dims       map[string]string
	Hist       *Histogram
}

// MetricExemplar points from a bad value to the session it came from.
type MetricExemplar struct {
	ProjectId  uuid.UUID         `json:"-"`
	Name       string            `json:"name"`
	RecordedAt time.Time         `json:"recordedAt"`
	Dims       map[string]string `json:"dims"`
	Value      float64           `json:"value"`
	SessionId  string            `json:"sessionId"`
	// Target is what the value is attributed to when the client knows: the element
	// behind a slow paint or interaction.
	Target string `json:"target,omitempty"`
}

// MetricSeriesSet is the series one metric of one project has in storage.
type MetricSeriesSet struct {
	ProjectId uuid.UUID
	Name      string
	Dims      []map[string]string
}

// MaxHotSeriesMetrics bounds how many metrics a hot series read returns in full.
const MaxHotSeriesMetrics = 200

type HistogramSeriesPoint struct {
	Timestamp time.Time
	Hist      *Histogram
	// GrainSeconds is the coarsest stored row merged into the point: 60, 3600 or 86400. A point of a grain
	// longer than the bucket holds everything of that longer period, see SpreadHistogramPoints.
	GrainSeconds int
}

// HistogramDimensionColumns maps the dimension keys of client metrics to the
// ClickHouse columns that hold them. Every other key lives in the tags map. The
// embedded backends keep all dimensions in one JSON object and do not use this.
var HistogramDimensionColumns = map[string]string{
	"service.version":             "release",
	"deployment.environment.name": "env",
	"os.name":                     "os",
	"os.major":                    "os_major",
	"browser.name":                "client",
	"browser.major":               "client_major",
	"device.form_factor":          "form_factor",
	"device.class":                "device_class",
	"network.type":                "network",
	"country":                     "country",
	"url.path":                    "view",
}

// HistogramDimensionColumnOrder is the column order of inserts and of the sort key.
var HistogramDimensionColumnOrder = []string{"release", "env", "os", "os_major", "client", "client_major", "form_factor", "device_class", "network", "country", "view"}

// SplitHistogramDims separates the dimensions with a column of their own (in
// HistogramDimensionColumnOrder) from the rest.
func SplitHistogramDims(dims map[string]string) ([]string, map[string]string) {
	columns := make(map[string]string, len(HistogramDimensionColumnOrder))
	tags := make(map[string]string)
	for key, value := range dims {
		if column, ok := HistogramDimensionColumns[key]; ok {
			columns[column] = value
		} else {
			tags[key] = value
		}
	}
	ordered := make([]string, len(HistogramDimensionColumnOrder))
	for i, column := range HistogramDimensionColumnOrder {
		ordered[i] = columns[column]
	}
	return ordered, tags
}

// JoinHistogramDims is the inverse of SplitHistogramDims; empty values are left out.
func JoinHistogramDims(columns []string, tags map[string]string) map[string]string {
	dims := make(map[string]string, len(tags)+4)
	for key, value := range tags {
		if value != "" {
			dims[key] = value
		}
	}
	for key, column := range HistogramDimensionColumns {
		for i, name := range HistogramDimensionColumnOrder {
			if name == column && i < len(columns) && columns[i] != "" {
				dims[key] = columns[i]
			}
		}
	}
	return dims
}

const (
	groupAll   = "__all__"
	groupEmpty = "(empty)"
)

// HistogramGroupKey builds a result key the way the metric point queries do, so the
// same response shape serves both kinds of metric.
func HistogramGroupKey(dims map[string]string, groupBy []string) string {
	if len(groupBy) == 0 {
		return groupAll
	}
	parts := make([]string, len(groupBy))
	for i, key := range groupBy {
		parts[i] = dims[key]
	}
	key := strings.Join(parts, GroupKeySeparator)
	if key == "" {
		return groupEmpty
	}
	return key
}

// HistogramSeries merges histogram rows into one histogram per group per time
// bucket. Rows may arrive in any order and any number of times per cell: merging is
// what the storage engines would do anyway.
//
// With more groups than maxGroups the ones with the most observations are kept, so a
// breakdown shows its busiest rows. Groups are collected up to a multiple of the cap
// first; past that, further groups are not looked at.
type HistogramSeries struct {
	interval  time.Duration
	groupBy   []string
	maxGroups int
	groups    map[string]*histogramGroup
	Truncated bool
}

type histogramGroup struct {
	buckets map[int64]*HistogramSeriesPoint
	count   float64
}

func NewHistogramSeries(intervalMinutes int, groupBy []string, maxGroups int) *HistogramSeries {
	if intervalMinutes < 1 {
		intervalMinutes = 1
	}
	return &HistogramSeries{
		interval:  time.Duration(intervalMinutes) * time.Minute,
		groupBy:   groupBy,
		maxGroups: maxGroups,
		groups:    make(map[string]*histogramGroup),
	}
}

// collectedGroups is how many groups are held while reading, before the busiest
// maxGroups are chosen.
func (s *HistogramSeries) collectedGroups() int {
	return max(2*s.maxGroups, s.maxGroups+2000)
}

func (s *HistogramSeries) Add(recordedAt time.Time, dims map[string]string, hist *Histogram) {
	s.AddGrain(recordedAt, HistogramGroupKey(dims, s.groupBy), hist, 0)
}

func (s *HistogramSeries) AddKeyed(recordedAt time.Time, key string, hist *Histogram) {
	s.AddGrain(recordedAt, key, hist, 0)
}

// AddGrain adds a row stored at the given grain (seconds; 0 when unknown).
func (s *HistogramSeries) AddGrain(recordedAt time.Time, key string, hist *Histogram, grainSeconds int) {
	group, ok := s.groups[key]
	if !ok {
		if s.maxGroups > 0 && len(s.groups) >= s.collectedGroups() {
			s.Truncated = true
			return
		}
		group = &histogramGroup{buckets: make(map[int64]*HistogramSeriesPoint)}
		s.groups[key] = group
	}
	seconds := int64(s.interval / time.Second)
	bucket := recordedAt.Unix() / seconds * seconds
	point, ok := group.buckets[bucket]
	if !ok {
		point = &HistogramSeriesPoint{Timestamp: time.Unix(bucket, 0).UTC(), Hist: NewHistogram()}
		group.buckets[bucket] = point
	}
	point.Hist.Merge(hist)
	point.GrainSeconds = max(point.GrainSeconds, grainSeconds)
	group.count += hist.Count
}

// GroupCount is how many groups were read so far, before Result keeps the busiest.
func (s *HistogramSeries) GroupCount() int {
	return len(s.groups)
}

func (s *HistogramSeries) Result() map[string][]HistogramSeriesPoint {
	keys := make([]string, 0, len(s.groups))
	for key := range s.groups {
		keys = append(keys, key)
	}
	if s.maxGroups > 0 && len(keys) > s.maxGroups {
		sort.Slice(keys, func(i, j int) bool {
			if s.groups[keys[i]].count != s.groups[keys[j]].count {
				return s.groups[keys[i]].count > s.groups[keys[j]].count
			}
			return keys[i] < keys[j]
		})
		keys = keys[:s.maxGroups]
		s.Truncated = true
	}
	result := make(map[string][]HistogramSeriesPoint, len(keys))
	for _, key := range keys {
		buckets := s.groups[key].buckets
		points := make([]HistogramSeriesPoint, 0, len(buckets))
		for _, point := range buckets {
			points = append(points, *point)
		}
		sort.Slice(points, func(i, j int) bool { return points[i].Timestamp.Before(points[j].Timestamp) })
		result[key] = points
	}
	return result
}

// SpreadHistogramPoints makes a chart of points whose stored grain is longer than the
// chart's bucket. An hour row lands in the :00 bucket of a 15 minute chart, which
// would draw four times the count there and nothing in the next three buckets, so it
// is divided evenly over the buckets its hour covers, inside [from, to]. Percentiles
// are unchanged by the division; counts, sums and rates come out per bucket.
//
// It is for charts only: anything that counts events (SLOs, alert thresholds) reads
// the points as stored.
func SpreadHistogramPoints(points []HistogramSeriesPoint, intervalMinutes int, from, to time.Time) []HistogramSeriesPoint {
	interval := int64(intervalMinutes) * 60
	if interval <= 0 {
		return points
	}
	coarse := false
	for _, point := range points {
		if int64(point.GrainSeconds) > interval {
			coarse = true
			break
		}
	}
	if !coarse {
		return points
	}
	merged := make(map[int64]*Histogram, len(points))
	add := func(bucket int64, hist *Histogram) {
		if existing, ok := merged[bucket]; ok {
			existing.Merge(hist)
			return
		}
		copied := NewHistogram()
		copied.Merge(hist)
		merged[bucket] = copied
	}
	first := from.Unix() / interval * interval
	for _, point := range points {
		start := point.Timestamp.Unix()
		grain := int64(point.GrainSeconds)
		if grain <= interval {
			add(start, point.Hist)
			continue
		}
		var buckets []int64
		for bucket := start; bucket < start+grain; bucket += interval {
			if bucket >= first && bucket <= to.Unix() {
				buckets = append(buckets, bucket)
			}
		}
		if len(buckets) == 0 {
			buckets = []int64{start}
		}
		share := point.Hist.Scaled(1 / float64(len(buckets)))
		for _, bucket := range buckets {
			add(bucket, share)
		}
	}
	out := make([]HistogramSeriesPoint, 0, len(merged))
	for bucket, hist := range merged {
		out = append(out, HistogramSeriesPoint{Timestamp: time.Unix(bucket, 0).UTC(), Hist: hist})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out
}

// MatchesDims reports whether a cell's dimensions satisfy every filter. A filter on
// an absent dimension only matches the empty string.
func MatchesDims(dims, filters map[string]string) bool {
	for key, want := range filters {
		if dims[key] != want {
			return false
		}
	}
	return true
}

// DiscoveredHistogramMetrics turns name -> dimension keys into the discovery shape,
// sorted by name with sorted keys.
func DiscoveredHistogramMetrics(keys map[string]map[string]struct{}) []models.DiscoveredMetric {
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	metrics := make([]models.DiscoveredMetric, 0, len(names))
	for _, name := range names {
		tagKeys := make([]string, 0, len(keys[name]))
		for key := range keys[name] {
			tagKeys = append(tagKeys, key)
		}
		sort.Strings(tagKeys)
		metrics = append(metrics, models.DiscoveredMetric{Name: name, TagKeys: tagKeys, MetricType: "histogram"})
	}
	return metrics
}
