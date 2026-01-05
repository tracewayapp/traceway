package models

import "time"

type DashboardTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type DashboardMetric struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Value     float64               `json:"value"`
	Unit      string                `json:"unit"`
	Trend     []DashboardTrendPoint `json:"trend"`
	Change24h float64               `json:"change24h"`
	Status    string                `json:"status"`
}

type DashboardResponse struct {
	Metrics     []DashboardMetric `json:"metrics"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

// TimeSeriesPoint is used internally for querying time-bucketed data
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}
