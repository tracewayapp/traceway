package controllers

import (
	"math"

	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// ChartSeriesResponse is a chart of numbers over time, for log attributes,
// span durations and span attributes. Present counts the matching rows that
// carry the value and Numeric those where it is a number; only numbers are
// charted.
type ChartSeriesResponse struct {
	IntervalSeconds int64                      `json:"intervalSeconds"`
	Present         int64                      `json:"present"`
	Numeric         int64                      `json:"numeric"`
	Buckets         []shared.ChartSeriesBucket `json:"buckets"`
}

// ChartDistributionResponse counts the numbers in bins next to their summary
// statistics. Scale is "log" when the bins cover equal ratios rather than equal
// widths, and BinWidth is then 0.
type ChartDistributionResponse struct {
	Present  int64             `json:"present"`
	Numeric  int64             `json:"numeric"`
	Integral bool              `json:"integral"`
	Min      float64           `json:"min"`
	Max      float64           `json:"max"`
	Avg      float64           `json:"avg"`
	Sum      float64           `json:"sum"`
	P50      float64           `json:"p50"`
	P95      float64           `json:"p95"`
	P99      float64           `json:"p99"`
	Scale    string            `json:"scale"`
	BinWidth float64           `json:"binWidth"`
	Bins     []shared.ChartBin `json:"bins"`
}

func chartBinCount(requested int) int {
	if requested == 0 {
		requested = shared.ChartDefaultBins
	}
	return min(max(requested, shared.ChartMinBins), shared.ChartMaxBins)
}

// chartDistribution builds a distribution from the whole-range statistics and
// the bin counts that binCounts reads for the binning it is given. allowLog
// lets values that span a wide range (durations) use log-scale bins.
func chartDistribution(stats []shared.ChartStatsRow, bins int, allowLog bool, binCounts func(shared.ChartBinning) ([]shared.ChartBinRow, error)) (ChartDistributionResponse, error) {
	response := ChartDistributionResponse{Scale: "linear", Bins: []shared.ChartBin{}}
	for _, row := range stats {
		response.Present += row.Present
		response.Numeric += row.Numeric
	}
	if len(stats) != 1 || stats[0].Numeric == 0 {
		return response, nil
	}
	row := stats[0]
	response.Integral = row.Integral == row.Numeric
	response.Min, response.Max, response.Sum = row.Min, row.Max, row.Sum
	response.Avg = row.Sum / float64(row.Numeric)
	response.P50, response.P95, response.P99 = row.P50, row.P95, row.P99
	binning := shared.NiceChartBins(row.Min, row.Max, bins, response.Integral)
	if allowLog && row.Max >= shared.ChartLogScaleRatio*math.Max(row.Min, shared.ChartLogScaleFloor) {
		binning = shared.NiceLogScaleBins(row.Min, row.Max, bins)
		response.Scale = "log"
	}
	rows, err := binCounts(binning)
	if err != nil {
		return response, err
	}
	if !binning.Log {
		response.BinWidth = binning.Width
	}
	response.Bins = shared.BuildChartBins(binning, rows)
	return response, nil
}
