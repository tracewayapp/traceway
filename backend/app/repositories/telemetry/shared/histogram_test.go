package shared

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"
)

func TestHistogramBucketContainsItsValue(t *testing.T) {
	for _, value := range []float64{0.001, 0.1, 1, 2.5, 100, 2500, 60000, 1e9} {
		low, high := histogramBucketBounds(HistogramBucketIndex(value))
		if value <= low*(1-1e-9) || value > high*(1+1e-9) {
			t.Errorf("%v not in (%v, %v]", value, low, high)
		}
	}
}

func TestHistogramQuantileStaysWithinFivePercent(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := make([]float64, 20000)
	h := NewHistogram()
	for i := range values {
		values[i] = math.Exp(rng.NormFloat64()*1.2 + 7)
		h.Observe(values[i], 1)
	}
	sort.Float64s(values)
	for _, q := range []float64{0.5, 0.75, 0.9, 0.95, 0.99} {
		want := values[int(q*float64(len(values)-1))]
		got := h.Quantile(q)
		if math.Abs(got-want)/want > 0.05 {
			t.Errorf("p%v = %v, want %v within 5%%", q*100, got, want)
		}
	}
	if h.Quantile(0) < values[0] || h.Quantile(1) > values[len(values)-1] {
		t.Errorf("quantiles left the recorded range")
	}
}

func TestHistogramMergeEqualsObservingEverything(t *testing.T) {
	a, b, all := NewHistogram(), NewHistogram(), NewHistogram()
	for i := 1; i <= 100; i++ {
		v := float64(i * 13)
		if i%2 == 0 {
			a.Observe(v, 1)
		} else {
			b.Observe(v, 1)
		}
		all.Observe(v, 1)
	}
	a.Merge(b)
	if a.Count != all.Count || a.Sum != all.Sum || a.Min != all.Min || a.Max != all.Max {
		t.Fatalf("merged %+v, want %+v", a, all)
	}
	for _, q := range []float64{0.5, 0.95} {
		if a.Quantile(q) != all.Quantile(q) {
			t.Errorf("p%v differs after merge", q*100)
		}
	}
}

func TestHistogramWeightsCorrectSampling(t *testing.T) {
	h := NewHistogram()
	h.Observe(100, 10)
	h.Observe(0, 10)
	if h.Count != 20 || h.Sum != 1000 || h.Zero != 10 {
		t.Fatalf("got count %v sum %v zero %v", h.Count, h.Sum, h.Zero)
	}
	if h.Observe(-1, 1) || h.Observe(math.NaN(), 1) || h.Observe(1, 0) {
		t.Fatal("accepted a value or weight a histogram cannot hold")
	}
}

func TestHistogramGoodRatio(t *testing.T) {
	h := NewHistogram()
	for i := 0; i < 75; i++ {
		h.Observe(1000, 1)
	}
	for i := 0; i < 25; i++ {
		h.Observe(6000, 1)
	}
	if got := h.Aggregate(AggregationGoodRatio, 2500); math.Abs(got-0.75) > 1e-9 {
		t.Errorf("good ratio = %v, want 0.75", got)
	}
	if got := h.Aggregate(AggregationGoodRatio, 10000); got != 1 {
		t.Errorf("good ratio above max = %v, want 1", got)
	}
}

func TestHistogramFromExponentialDownscalesExactly(t *testing.T) {
	const scale = 5
	want := NewHistogram()
	counts := map[int32]uint64{}
	var low, high int32
	first := true
	for _, v := range []float64{12, 13, 14, 300, 305, 9000} {
		want.Observe(v, 1)
		index := int32(math.Ceil(math.Log2(v)*(1<<scale))) - 1
		counts[index]++
		if first || index < low {
			low = index
		}
		if first || index > high {
			high = index
		}
		first = false
	}
	dense := make([]uint64, high-low+1)
	for index, c := range counts {
		dense[index-low] = c
	}
	got := HistogramFromExponential(ExponentialHistogramSource{Scale: scale, Offset: low, BucketCnts: dense, Sum: want.Sum, HasSum: true, Min: 12, Max: 9000, HasMinMax: true}, 1)
	if got.Count != want.Count || len(got.Buckets) != len(want.Buckets) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for index, c := range want.Buckets {
		if got.Buckets[index] != c {
			t.Errorf("bucket %d = %v, want %v", index, got.Buckets[index], c)
		}
	}
}

func TestHistogramFromExponentialCoarserSource(t *testing.T) {
	// Scale 0: bucket 9 is (512, 1024].
	got := HistogramFromExponential(ExponentialHistogramSource{Scale: 0, Offset: 9, BucketCnts: []uint64{4}}, 1)
	if got.Count != 4 {
		t.Fatalf("count = %v", got.Count)
	}
	if p50 := got.Quantile(0.5); p50 <= 512 || p50 > 1024 {
		t.Errorf("p50 = %v, want inside (512, 1024]", p50)
	}
}

func TestHistogramFromExplicitBounds(t *testing.T) {
	bounds := []float64{5, 10, 25, 50, 100, 250, 500, 1000}
	counts := []uint64{0, 10, 30, 40, 15, 4, 1, 0, 0}
	h := HistogramFromExplicit(bounds, counts, 3000, true, 6, 400, true, 1)
	if h.Count != 100 || h.Sum != 3000 {
		t.Fatalf("count %v sum %v", h.Count, h.Sum)
	}
	if p50 := h.Quantile(0.5); p50 < 25 || p50 > 50 {
		t.Errorf("p50 = %v, want in the 25..50 bucket", p50)
	}
	if h.Min != 6 || h.Max != 400 {
		t.Errorf("min/max = %v/%v", h.Min, h.Max)
	}
}

func TestHistogramSubGivesTheDeltaOfACumulativeSeries(t *testing.T) {
	previous, current := NewHistogram(), NewHistogram()
	for _, v := range []float64{10, 20} {
		previous.Observe(v, 1)
		current.Observe(v, 1)
	}
	current.Observe(40, 1)
	delta, ok := current.Sub(previous)
	if !ok || delta.Count != 1 || delta.Sum != 40 {
		t.Fatalf("delta %+v ok %v", delta, ok)
	}
	if _, ok := previous.Sub(current); ok {
		t.Fatal("a reset was taken for a delta")
	}
}

func TestBucketsRoundTrip(t *testing.T) {
	h := NewHistogram()
	for _, v := range []float64{0.5, 3, 3, 900} {
		h.Observe(v, 2.5)
	}
	decoded, err := DecodeBuckets(h.EncodeBuckets())
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(h.Buckets) {
		t.Fatalf("decoded %v, want %v", decoded, h.Buckets)
	}
	for index, c := range h.Buckets {
		if decoded[index] != c {
			t.Errorf("bucket %d = %v, want %v", index, decoded[index], c)
		}
	}
	indexes, counts := h.BucketArrays()
	if len(indexes) != 3 || !sort.SliceIsSorted(indexes, func(i, j int) bool { return indexes[i] < indexes[j] }) || len(counts) != 3 {
		t.Errorf("arrays %v %v", indexes, counts)
	}
}

func TestHistogramSeriesGroupsAndBuckets(t *testing.T) {
	base := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	series := NewHistogramSeries(5, []string{"service.version"}, 2)
	add := func(minute int, release string, value float64) {
		h := NewHistogram()
		h.Observe(value, 1)
		series.Add(base.Add(time.Duration(minute)*time.Minute), map[string]string{"service.version": release}, h)
	}
	add(0, "1.0", 100)
	add(4, "1.0", 300)
	add(5, "1.0", 500)
	add(1, "", 50)
	add(2, "2.0", 70)
	result := series.Result()
	if len(result) != 2 || !series.Truncated {
		t.Fatalf("groups %d truncated %v", len(result), series.Truncated)
	}
	points := result["1.0"]
	if len(points) != 2 || points[0].Hist.Count != 2 || points[1].Hist.Count != 1 || !points[0].Timestamp.Equal(base) {
		t.Fatalf("points %+v", points)
	}
	if _, ok := result["(empty)"]; !ok {
		t.Errorf("an empty group value should be keyed (empty)")
	}
}

func TestHistogramDimsSplitAndJoin(t *testing.T) {
	dims := map[string]string{"service.version": "4.2", "url.path": "/orders/:id", "rating": "poor"}
	columns, tags := SplitHistogramDims(dims)
	if columns[0] != "4.2" || columns[len(columns)-1] != "/orders/:id" || tags["rating"] != "poor" || len(tags) != 1 {
		t.Fatalf("columns %v tags %v", columns, tags)
	}
	joined := JoinHistogramDims(columns, tags)
	if len(joined) != 3 || joined["url.path"] != "/orders/:id" {
		t.Errorf("joined %v", joined)
	}
}
