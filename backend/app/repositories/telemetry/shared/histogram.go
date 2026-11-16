package shared

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
)

// HistogramScale is the one scale every stored histogram uses: base-2 exponential
// buckets, 8 per doubling. Bucket i holds the values in (2^(i/8), 2^((i+1)/8)], about
// 9% wide, so a percentile read from it is within about 4.5% of the true value. One
// scale everywhere is what lets histograms from different clients, minutes and
// instances add up bucket by bucket.
const HistogramScale = 3

const (
	histogramBucketsPerDoubling = 1 << HistogramScale
	histogramZeroThreshold      = 1e-6
	histogramMaxValue           = 1e12
)

// Histogram counts are float64 because a sampled client's observations are weighted
// by 1/rate before they are added.
type Histogram struct {
	Count   float64
	Sum     float64
	Min     float64
	Max     float64
	Zero    float64
	Buckets map[int32]float64
}

func NewHistogram() *Histogram {
	return &Histogram{Buckets: make(map[int32]float64)}
}

func HistogramBucketIndex(value float64) int32 {
	return int32(math.Ceil(math.Log2(value)*histogramBucketsPerDoubling)) - 1
}

func histogramBucketBounds(index int32) (float64, float64) {
	return math.Pow(2, float64(index)/histogramBucketsPerDoubling), math.Pow(2, float64(index+1)/histogramBucketsPerDoubling)
}

func (h *Histogram) noteRange(low, high float64) {
	if h.Count == 0 || low < h.Min {
		h.Min = low
	}
	if h.Count == 0 || high > h.Max {
		h.Max = high
	}
}

// Observe adds one value with the given weight. It reports false for a value a
// histogram cannot hold.
func (h *Histogram) Observe(value, weight float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || !(weight > 0) {
		return false
	}
	if value > histogramMaxValue {
		value = histogramMaxValue
	}
	h.noteRange(value, value)
	h.Count += weight
	h.Sum += value * weight
	if value < histogramZeroThreshold {
		h.Zero += weight
	} else {
		h.Buckets[HistogramBucketIndex(value)] += weight
	}
	return true
}

func (h *Histogram) Merge(other *Histogram) {
	if other == nil || other.Count <= 0 {
		return
	}
	h.noteRange(other.Min, other.Max)
	h.Count += other.Count
	h.Sum += other.Sum
	h.Zero += other.Zero
	for index, count := range other.Buckets {
		h.Buckets[index] += count
	}
}

// Scaled returns a copy with every count multiplied by factor. Quantiles do not
// change; it is how one population is reweighted to another's mix before a merge.
func (h *Histogram) Scaled(factor float64) *Histogram {
	out := &Histogram{Count: h.Count * factor, Sum: h.Sum * factor, Min: h.Min, Max: h.Max, Zero: h.Zero * factor, Buckets: make(map[int32]float64, len(h.Buckets))}
	for index, count := range h.Buckets {
		out.Buckets[index] = count * factor
	}
	return out
}

// QuantileInterval is the range the q-quantile of the underlying population lies in
// with about 95% confidence, from the binomial spread of the rank. It is as wide as
// the sample is small, which is what lets two releases be compared honestly.
func (h *Histogram) QuantileInterval(q float64) (low, high float64) {
	if h.Count <= 0 {
		return 0, 0
	}
	spread := 1.96 * math.Sqrt(q*(1-q)/h.Count)
	return h.Quantile(math.Max(0, q-spread)), h.Quantile(math.Min(1, q+spread))
}

// Sub returns what was added to h since previous, for a cumulative series. It
// reports false when the series was reset (a count went down), in which case the
// caller starts over from h.
func (h *Histogram) Sub(previous *Histogram) (*Histogram, bool) {
	if h.Count < previous.Count || h.Zero < previous.Zero {
		return nil, false
	}
	delta := NewHistogram()
	delta.Count = h.Count - previous.Count
	delta.Sum = h.Sum - previous.Sum
	delta.Zero = h.Zero - previous.Zero
	delta.Min, delta.Max = h.Min, h.Max
	for index, count := range h.Buckets {
		d := count - previous.Buckets[index]
		if d < 0 {
			return nil, false
		}
		if d > 0 {
			delta.Buckets[index] = d
		}
	}
	return delta, true
}

// ExponentialHistogramSource is one OTLP ExponentialHistogram data point, already
// multiplied out of protobuf.
type ExponentialHistogramSource struct {
	Scale      int32
	Count      float64
	Sum        float64
	HasSum     bool
	Min, Max   float64
	HasMinMax  bool
	ZeroCount  float64
	Offset     int32
	BucketCnts []uint64
}

// HistogramFromExponential converts an OTLP exponential histogram to the stored
// scale. A finer source is merged down exactly (bucket i at scale s lies inside
// bucket i>>(s-3)); a coarser one is placed in the stored bucket at the middle of
// each source bucket, which is as precise as the source allows.
func HistogramFromExponential(src ExponentialHistogramSource, weight float64) *Histogram {
	h := NewHistogram()
	if !(weight > 0) {
		return h
	}
	var bucketTotal float64
	for i, c := range src.BucketCnts {
		if c == 0 {
			continue
		}
		index := src.Offset + int32(i)
		var stored int32
		switch shift := src.Scale - HistogramScale; {
		case shift >= 0:
			if shift > 31 {
				shift = 31
			}
			stored = index >> uint(shift)
		default:
			up := uint(-shift)
			if up > 20 {
				continue
			}
			stored = index<<up + int32((1<<up-1)/2)
		}
		h.Buckets[stored] += float64(c) * weight
		bucketTotal += float64(c)
	}
	h.Zero = src.ZeroCount * weight
	h.Count = (bucketTotal + src.ZeroCount) * weight
	if h.Count <= 0 {
		return NewHistogram()
	}
	h.finishFromBuckets(src.Sum, src.HasSum, src.Min, src.Max, src.HasMinMax, weight)
	return h
}

// HistogramFromExplicit converts an OTLP explicit-bounds histogram. Bucket i holds
// (bounds[i-1], bounds[i]]; its count goes to the stored bucket at its geometric
// middle, and the two open-ended buckets to their one known bound.
func HistogramFromExplicit(bounds []float64, counts []uint64, sum float64, hasSum bool, min, max float64, hasMinMax bool, weight float64) *Histogram {
	h := NewHistogram()
	if !(weight > 0) {
		return h
	}
	for i, c := range counts {
		if c == 0 {
			continue
		}
		var value float64
		switch {
		case len(bounds) == 0:
			value = 0
		case i == 0:
			value = bounds[0]
			if hasMinMax && min > 0 && min < value {
				value = math.Sqrt(min * value)
			}
		case i >= len(bounds):
			value = bounds[len(bounds)-1]
			if hasMinMax && max > value {
				value = math.Sqrt(value * max)
			}
		default:
			low, high := bounds[i-1], bounds[i]
			if low > 0 {
				value = math.Sqrt(low * high)
			} else {
				value = high / 2
			}
		}
		n := float64(c) * weight
		h.Count += n
		if value < histogramZeroThreshold {
			h.Zero += n
		} else {
			h.Buckets[HistogramBucketIndex(math.Min(value, histogramMaxValue))] += n
		}
	}
	if h.Count <= 0 {
		return NewHistogram()
	}
	h.finishFromBuckets(sum, hasSum, min, max, hasMinMax, weight)
	return h
}

func (h *Histogram) finishFromBuckets(sum float64, hasSum bool, min, max float64, hasMinMax bool, weight float64) {
	low, high := h.bucketRange()
	if hasMinMax && min <= max {
		low, high = min, max
	}
	h.Min, h.Max = low, high
	if hasSum {
		h.Sum = sum * weight
		return
	}
	for index, count := range h.Buckets {
		lo, hi := histogramBucketBounds(index)
		h.Sum += math.Sqrt(lo*hi) * count
	}
}

func (h *Histogram) bucketRange() (float64, float64) {
	if len(h.Buckets) == 0 {
		return 0, 0
	}
	first := true
	var lowIndex, highIndex int32
	for index := range h.Buckets {
		if first || index < lowIndex {
			lowIndex = index
		}
		if first || index > highIndex {
			highIndex = index
		}
		first = false
	}
	low, _ := histogramBucketBounds(lowIndex)
	_, high := histogramBucketBounds(highIndex)
	if h.Zero > 0 {
		low = 0
	}
	return low, high
}

func (h *Histogram) sortedIndexes() []int32 {
	indexes := make([]int32, 0, len(h.Buckets))
	for index := range h.Buckets {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
	return indexes
}

// Quantile reads the q-quantile (0..1), interpolating on a log scale inside the
// bucket and never leaving the recorded min and max.
func (h *Histogram) Quantile(q float64) float64 {
	if h.Count <= 0 {
		return 0
	}
	rank := q * h.Count
	if rank <= h.Zero {
		return math.Max(0, math.Min(h.Min, h.Max))
	}
	seen := h.Zero
	value := h.Max
	for _, index := range h.sortedIndexes() {
		count := h.Buckets[index]
		if seen+count >= rank {
			low, high := histogramBucketBounds(index)
			value = low * math.Pow(high/low, (rank-seen)/count)
			break
		}
		seen += count
	}
	return math.Min(math.Max(value, h.Min), h.Max)
}

// CountAtOrBelow is the number of observations at or under threshold: the "good"
// side of a rated distribution. A bucket the threshold cuts through is split on a
// log scale.
func (h *Histogram) CountAtOrBelow(threshold float64) float64 {
	if h.Count <= 0 || threshold < 0 {
		return 0
	}
	if threshold >= h.Max {
		return h.Count
	}
	good := h.Zero
	for index, count := range h.Buckets {
		low, high := histogramBucketBounds(index)
		switch {
		case threshold >= high:
			good += count
		case threshold > low:
			good += count * math.Log(threshold/low) / math.Log(high/low)
		}
	}
	return math.Min(good, h.Count)
}

// HistogramAggregations are the aggregations a histogram metric answers.
var HistogramAggregations = map[string]float64{"p50": 0.5, "p75": 0.75, "p90": 0.9, "p95": 0.95, "p99": 0.99}

const AggregationGoodRatio = "good_ratio"

func IsHistogramOnlyAggregation(aggregation string) bool {
	_, ok := HistogramAggregations[aggregation]
	return ok || aggregation == AggregationGoodRatio
}

// Aggregate reads one number from the histogram. good_ratio is a fraction (0..1) of
// observations at or under threshold.
func (h *Histogram) Aggregate(aggregation string, threshold float64) float64 {
	if h.Count <= 0 {
		return 0
	}
	if q, ok := HistogramAggregations[aggregation]; ok {
		return h.Quantile(q)
	}
	switch aggregation {
	case AggregationGoodRatio:
		return h.CountAtOrBelow(threshold) / h.Count
	case "min":
		return h.Min
	case "max":
		return h.Max
	case "sum":
		return h.Sum
	case "count":
		return h.Count
	default:
		return h.Sum / h.Count
	}
}

// EncodeBuckets renders the buckets as the JSON object the embedded backends store.
func (h *Histogram) EncodeBuckets() string {
	out := make(map[string]float64, len(h.Buckets))
	for index, count := range h.Buckets {
		out[strconv.Itoa(int(index))] = count
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func DecodeBuckets(raw string) (map[int32]float64, error) {
	buckets := make(map[int32]float64)
	if raw == "" {
		return buckets, nil
	}
	var in map[string]float64
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		return nil, err
	}
	for key, count := range in {
		index, err := strconv.Atoi(key)
		if err != nil {
			return nil, err
		}
		buckets[int32(index)] = count
	}
	return buckets, nil
}

// BucketArrays renders the buckets as the two parallel arrays ClickHouse stores,
// sorted by index.
func (h *Histogram) BucketArrays() ([]int32, []float64) {
	indexes := h.sortedIndexes()
	counts := make([]float64, len(indexes))
	for i, index := range indexes {
		counts[i] = h.Buckets[index]
	}
	return indexes, counts
}
