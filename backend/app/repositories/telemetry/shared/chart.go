package shared

import (
	"math"
	"sort"
	"time"
)

// ChartNumberPattern is the shape an attribute value (of a log or a span) must
// have to be charted: a JSON number. Every backend applies the same grammar
// (SQLite through json_valid/json_type, ClickHouse and DuckDB through this RE2
// pattern), so "007", " 12", "1." and "NaN" are skipped everywhere alike. It is
// inlined into the SQL and spells "optional" as {0,1}: clickhouse-go would bind
// a literal ? as a parameter.
const ChartNumberPattern = `^-{0,1}(0|[1-9][0-9]*)([.][0-9]+){0,1}([eE][-+]{0,1}[0-9]+){0,1}$`

// ChartNumberLimit is the largest magnitude a charted number may have. A value
// that matches the pattern can still be too large for a float64 ("1e999", or an
// id such as "4e98213"): it parses as infinity, which no bin, sum or JSON
// response can hold, so every backend leaves it out like any other text.
const ChartNumberLimit = "1.7976931348623157e308"

// ChartQuantiles are the percentiles every stats row carries, in the
// order the backends return them.
var ChartQuantiles = [3]float64{0.5, 0.95, 0.99}

// ChartStatsRow summarises the values charted in one bucket. Present counts
// the matching rows that carry the value, Numeric those where it is a number,
// Integral those whose number is a whole one. Bucket is the bucket start in Unix
// seconds, or 0 for a whole-range read.
type ChartStatsRow struct {
	Bucket   int64
	Present  int64
	Numeric  int64
	Integral int64
	Sum      float64
	Min      float64
	Max      float64
	P50      float64
	P95      float64
	P99      float64
}

// ChartBinRow counts the numeric values that fall into bin Index of
// ChartBinning.
type ChartBinRow struct {
	Index int
	Count int64
}

// ChartBinning is the equal-width split of [Start, Start+Width*Count) a
// distribution is counted in. With Log, the split is of log10 of the values:
// every bin covers the same ratio, which keeps a latency distribution readable
// when a few requests take a hundred times longer than the rest.
type ChartBinning struct {
	Start float64
	Width float64
	Count int
	Log   bool
}

const (
	ChartDefaultBins = 40
	ChartMinBins     = 5
	ChartMaxBins     = 200
)

// NiceChartBins picks round bin edges near the target count that cover
// [min, max]. Whole numbers get whole-number bins, so a counter never shows a
// bin that no value can fall into.
func NiceChartBins(min, max float64, target int, integral bool) ChartBinning {
	if target < 1 {
		target = 1
	}
	if !(max > min) {
		width := 1.0
		start := min
		if integral {
			start = math.Floor(min)
		} else {
			start = min - width/2
		}
		return ChartBinning{Start: start, Width: width, Count: 1}
	}
	if math.IsInf(max-min, 0) {
		// Values almost as far apart as a float64 reaches: the range itself
		// overflows, so the bins are taken from the halves.
		return ChartBinning{Start: min, Width: max/float64(target) - min/float64(target), Count: target}
	}
	width := niceStep((max - min) / float64(target))
	if integral && width < 1 {
		width = 1
	}
	start := math.Floor(min/width) * width
	count := int(math.Floor((max-start)/width)) + 1
	if count < 1 {
		count = 1
	}
	return ChartBinning{Start: start, Width: width, Count: count}
}

// ChartLogScaleRatio is how far apart the smallest and the largest duration
// must be for a duration distribution to use log-scale bins.
const ChartLogScaleRatio = 20

// ChartLogScaleFloor is the smallest value a log-scale binning starts at, so a
// span that took no measurable time still lands in the first bin.
const ChartLogScaleFloor = 0.001

// NiceLogScaleBins splits [min, max] into bins of equal ratio, with as many bins
// per decade as fit the target (20, 10, 5, 4, 2 or 1), so the edges repeat the
// same steps in every decade.
func NiceLogScaleBins(min, max float64, target int) ChartBinning {
	if target < 1 {
		target = 1
	}
	lo := math.Log10(math.Max(min, ChartLogScaleFloor))
	hi := math.Log10(math.Max(max, ChartLogScaleFloor))
	var binning ChartBinning
	for _, perDecade := range []float64{20, 10, 5, 4, 2, 1} {
		start := math.Floor(lo*perDecade) / perDecade
		count := int(math.Floor((hi-start)*perDecade+1e-9)) + 1
		binning = ChartBinning{Start: start, Width: 1 / perDecade, Count: count, Log: true}
		if count <= target {
			break
		}
	}
	return binning
}

// Edge is where bin i starts (i == Count is where the last one ends), in the
// units of the values.
func (b ChartBinning) Edge(i int) float64 {
	edge := b.Start + float64(i)*b.Width
	if b.Log {
		return math.Pow(10, edge)
	}
	return edge
}

// BinExpression is the SQL that puts the value expression v into a bin: the
// floor of where it lies, before clamping. greatest names the backend's
// two-argument maximum (SQLite spells it max). Its placeholders come first, in
// the order of args.
func (b ChartBinning) BinExpression(v, greatest string) (string, []any) {
	if b.Log {
		return "floor((log10(" + greatest + "(" + v + ", ?)) - ?) / ?)", []any{b.Edge(0), b.Start, b.Width}
	}
	return "floor((" + v + " - ?) / ?)", []any{b.Start, b.Width}
}

func niceStep(raw float64) float64 {
	if raw <= 0 || math.IsInf(raw, 0) || math.IsNaN(raw) {
		return 1
	}
	exponent := math.Floor(math.Log10(raw))
	base := math.Pow(10, exponent)
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if raw <= m*base*(1+1e-9) {
			return m * base
		}
	}
	return 10 * base
}

// Index is the bin a value falls into; the bin expression of each backend
// computes the same thing in SQL.
func (b ChartBinning) Index(v float64) int {
	if b.Log {
		v = math.Log10(math.Max(v, b.Edge(0)))
	}
	i := int(math.Floor((v - b.Start) / b.Width))
	if i < 0 {
		return 0
	}
	if i >= b.Count {
		return b.Count - 1
	}
	return i
}

type ChartSeriesBucket struct {
	Start time.Time `json:"start"`
	Count int64     `json:"count"`
	Sum   float64   `json:"sum"`
	Avg   float64   `json:"avg"`
	Min   float64   `json:"min"`
	Max   float64   `json:"max"`
	P50   float64   `json:"p50"`
	P95   float64   `json:"p95"`
	P99   float64   `json:"p99"`
}

// FiniteChartStats keeps every statistic inside what JSON can carry. Each
// charted value is finite, but a sum of them, or a percentile interpolated
// between two of them, can still overflow.
func FiniteChartStats(rows []ChartStatsRow) {
	for i := range rows {
		r := &rows[i]
		for _, v := range []*float64{&r.Sum, &r.Min, &r.Max, &r.P50, &r.P95, &r.P99} {
			switch {
			case math.IsNaN(*v):
				*v = 0
			case math.IsInf(*v, 1):
				*v = math.MaxFloat64
			case math.IsInf(*v, -1):
				*v = -math.MaxFloat64
			}
		}
	}
}

// BuildChartSeries keeps the buckets holding at least one number, in time
// order, and totals the counts of all of them.
func BuildChartSeries(rows []ChartStatsRow) (buckets []ChartSeriesBucket, present, numeric int64) {
	FiniteChartStats(rows)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Bucket < rows[j].Bucket })
	buckets = []ChartSeriesBucket{}
	for _, r := range rows {
		present += r.Present
		numeric += r.Numeric
		if r.Numeric == 0 {
			continue
		}
		buckets = append(buckets, ChartSeriesBucket{
			Start: time.Unix(r.Bucket, 0).UTC(),
			Count: r.Numeric,
			Sum:   r.Sum,
			Avg:   r.Sum / float64(r.Numeric),
			Min:   r.Min,
			Max:   r.Max,
			P50:   r.P50,
			P95:   r.P95,
			P99:   r.P99,
		})
	}
	return buckets, present, numeric
}

type ChartBin struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Count int64   `json:"count"`
}

// BuildChartBins returns every bin of the binning, empty ones included,
// so the chart's x axis is continuous.
func BuildChartBins(binning ChartBinning, rows []ChartBinRow) []ChartBin {
	bins := make([]ChartBin, binning.Count)
	for i := range bins {
		bins[i].Start = binning.Edge(i)
		bins[i].End = binning.Edge(i + 1)
	}
	for _, r := range rows {
		if r.Index >= 0 && r.Index < len(bins) {
			bins[r.Index].Count += r.Count
		}
	}
	return bins
}

// ChartQuantileRow is one value at a sorted position of its bucket: the
// SQLite backend reads the two values around every quantile position and
// interpolates them like DuckDB's quantile_cont and ClickHouse's quantile.
type ChartQuantileRow struct {
	Bucket   int64
	N        int64
	Position int64
	Value    float64
}

// ChartQuantilePositions are the sorted positions (0-based) an
// interpolated quantile of n values reads: floor((n-1)q) and the one after it.
func ChartQuantilePositions(n int64, q float64) (int64, int64, float64) {
	h := float64(n-1) * q
	lo := int64(math.Floor(h))
	return lo, lo + 1, h - float64(lo)
}

// ApplyChartQuantiles fills the percentiles of the stats rows from the
// values read at their positions. The positions are those of the count the
// values were read with (N), not of the row's own: the two come from separate
// reads, and a record stored between them moves every position.
func ApplyChartQuantiles(rows []ChartStatsRow, values []ChartQuantileRow) {
	type key struct{ bucket, position int64 }
	at := make(map[key]float64, len(values))
	counts := make(map[int64]int64)
	for _, v := range values {
		at[key{v.Bucket, v.Position}] = v.Value
		counts[v.Bucket] = v.N
	}
	for i := range rows {
		n := counts[rows[i].Bucket]
		if n == 0 || rows[i].Numeric == 0 {
			continue
		}
		quantile := func(q float64) float64 {
			lo, hi, frac := ChartQuantilePositions(n, q)
			low := at[key{rows[i].Bucket, lo}]
			if hi >= n || frac == 0 {
				return low
			}
			return low + frac*(at[key{rows[i].Bucket, hi}]-low)
		}
		rows[i].P50 = quantile(ChartQuantiles[0])
		rows[i].P95 = quantile(ChartQuantiles[1])
		rows[i].P99 = quantile(ChartQuantiles[2])
	}
}
