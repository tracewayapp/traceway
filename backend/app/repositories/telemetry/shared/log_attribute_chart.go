package shared

import (
	"math"
	"sort"
	"time"
)

// LogAttributeNumberPattern is the shape a log attribute value must have to be
// charted: a JSON number. Every backend applies the same grammar (SQLite through
// json_valid/json_type, ClickHouse and DuckDB through this RE2 pattern), so
// "007", " 12", "1." and "NaN" are skipped everywhere alike. It is inlined into
// the SQL and spells "optional" as {0,1}: clickhouse-go would bind a literal ?
// as a parameter.
const LogAttributeNumberPattern = `^-{0,1}(0|[1-9][0-9]*)([.][0-9]+){0,1}([eE][-+]{0,1}[0-9]+){0,1}$`

// LogAttributeQuantiles are the percentiles every stats row carries, in the
// order the backends return them.
var LogAttributeQuantiles = [3]float64{0.5, 0.95, 0.99}

// LogAttributeTarget names one attribute of the logs a search matches. Scope is
// "resource", "scope" or "log", like LogAttributeFilter.
type LogAttributeTarget struct {
	Scope string
	Key   string
}

// LogAttributeStatsRow summarises the values of one attribute in one bucket.
// Present counts the matching logs that carry the attribute, Numeric those whose
// value is a number, Integral those whose number is a whole one. Bucket is the
// bucket start in Unix seconds, or 0 for a whole-range read.
type LogAttributeStatsRow struct {
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

// LogAttributeBinRow counts the numeric values that fall into bin Index of
// LogAttributeBinning.
type LogAttributeBinRow struct {
	Index int
	Count int64
}

// LogAttributeBinning is the equal-width split of [Start, Start+Width*Count) a
// distribution is counted in.
type LogAttributeBinning struct {
	Start float64
	Width float64
	Count int
}

const (
	LogAttributeDefaultBins = 40
	LogAttributeMinBins     = 5
	LogAttributeMaxBins     = 200
)

// NiceLogAttributeBins picks round bin edges near the target count that cover
// [min, max]. Whole numbers get whole-number bins, so a counter never shows a
// bin that no value can fall into.
func NiceLogAttributeBins(min, max float64, target int, integral bool) LogAttributeBinning {
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
		return LogAttributeBinning{Start: start, Width: width, Count: 1}
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
	return LogAttributeBinning{Start: start, Width: width, Count: count}
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
func (b LogAttributeBinning) Index(v float64) int {
	i := int(math.Floor((v - b.Start) / b.Width))
	if i < 0 {
		return 0
	}
	if i >= b.Count {
		return b.Count - 1
	}
	return i
}

type LogAttributeSeriesBucket struct {
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

// BuildLogAttributeSeries keeps the buckets holding at least one number, in time
// order, and totals the counts of all of them.
func BuildLogAttributeSeries(rows []LogAttributeStatsRow) (buckets []LogAttributeSeriesBucket, present, numeric int64) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].Bucket < rows[j].Bucket })
	buckets = []LogAttributeSeriesBucket{}
	for _, r := range rows {
		present += r.Present
		numeric += r.Numeric
		if r.Numeric == 0 {
			continue
		}
		buckets = append(buckets, LogAttributeSeriesBucket{
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

type LogAttributeBin struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Count int64   `json:"count"`
}

// BuildLogAttributeBins returns every bin of the binning, empty ones included,
// so the chart's x axis is continuous.
func BuildLogAttributeBins(binning LogAttributeBinning, rows []LogAttributeBinRow) []LogAttributeBin {
	bins := make([]LogAttributeBin, binning.Count)
	for i := range bins {
		bins[i].Start = binning.Start + float64(i)*binning.Width
		bins[i].End = bins[i].Start + binning.Width
	}
	for _, r := range rows {
		if r.Index >= 0 && r.Index < len(bins) {
			bins[r.Index].Count += r.Count
		}
	}
	return bins
}

// LogAttributeQuantileRow is one value at a sorted position of its bucket: the
// SQLite backend reads the two values around every quantile position and
// interpolates them like DuckDB's quantile_cont and ClickHouse's quantile.
type LogAttributeQuantileRow struct {
	Bucket   int64
	N        int64
	Position int64
	Value    float64
}

// LogAttributeQuantilePositions are the sorted positions (0-based) an
// interpolated quantile of n values reads: floor((n-1)q) and the one after it.
func LogAttributeQuantilePositions(n int64, q float64) (int64, int64, float64) {
	h := float64(n-1) * q
	lo := int64(math.Floor(h))
	return lo, lo + 1, h - float64(lo)
}

// ApplyLogAttributeQuantiles fills the percentiles of the stats rows from the
// values read at their positions.
func ApplyLogAttributeQuantiles(rows []LogAttributeStatsRow, values []LogAttributeQuantileRow) {
	type key struct{ bucket, position int64 }
	at := make(map[key]float64, len(values))
	for _, v := range values {
		at[key{v.Bucket, v.Position}] = v.Value
	}
	for i := range rows {
		n := rows[i].Numeric
		if n == 0 {
			continue
		}
		quantile := func(q float64) float64 {
			lo, hi, frac := LogAttributeQuantilePositions(n, q)
			low := at[key{rows[i].Bucket, lo}]
			if hi >= n || frac == 0 {
				return low
			}
			return low + frac*(at[key{rows[i].Bucket, hi}]-low)
		}
		rows[i].P50 = quantile(LogAttributeQuantiles[0])
		rows[i].P95 = quantile(LogAttributeQuantiles[1])
		rows[i].P99 = quantile(LogAttributeQuantiles[2])
	}
}
