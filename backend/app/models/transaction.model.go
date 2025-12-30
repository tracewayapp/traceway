package models

import "time"

type Transaction struct {
	Id string `json:"id" ch:"id"`
	// endpoint is the route from the router/does not contain actual params so it's safe to group on it
	Endpoint   string        `json:"endpoint" ch:"endpoint"`
	Duration   time.Duration `json:"duration" ch:"duration"`
	RecordedAt time.Time     `json:"recordedAt" ch:"recorded_at"`
	StatusCode int           `json:"statusCode" ch:"status_code"`
	BodySize   int           `json:"bodySize" ch:"body_size"`
	ClientIP   string        `json:"clientIP" ch:"client_ip"`
}
