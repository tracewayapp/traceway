package models

import "time"

type Transaction struct {
	Id string `json:"id" ch:"id"`
	// endpoint is the route from the router/does not contain actual params so it's safe to group on it
	Endpoint   string        `json:"endpoint" ch:"endpoint"`
	Duration   time.Duration `json:"duration" ch:"duration"`
	RecordedAt time.Time     `json:"recordedAt" ch:"recorded_at"`
	StatusCode int32         `json:"statusCode" ch:"status_code"`
	BodySize   int32         `json:"bodySize" ch:"body_size"`
	ClientIP   string        `json:"clientIP" ch:"client_ip"`
}

type EndpointStats struct {
	Endpoint    string        `json:"endpoint"`
	Count       uint64        `json:"count"`
	P50Duration time.Duration `json:"p50Duration"`
	P95Duration time.Duration `json:"p95Duration"`
	AvgDuration time.Duration `json:"avgDuration"`
	LastSeen    time.Time     `json:"lastSeen"`
}
