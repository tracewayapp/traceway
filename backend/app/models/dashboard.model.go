package models

import "time"

type DashboardTrendPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ServerMetricTrend represents trend data for a single server
type ServerMetricTrend struct {
	ServerName string                `json:"serverName"`
	Value      float64               `json:"value"`
	Trend      []DashboardTrendPoint `json:"trend"`
}

type DashboardMetric struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Value     float64               `json:"value"`
	Unit      string                `json:"unit"`
	Trend     []DashboardTrendPoint `json:"trend"`
	Servers   []ServerMetricTrend   `json:"servers,omitempty"`
	Change24h float64               `json:"change24h"`
	Status    string                `json:"status"`
}

type DashboardResponse struct {
	Metrics          []DashboardMetric `json:"metrics"`
	AvailableServers []string          `json:"availableServers"`
	LastUpdated      time.Time         `json:"lastUpdated"`
}

// TimeSeriesPoint is used internally for querying time-bucketed data
type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}
