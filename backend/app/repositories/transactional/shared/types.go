package shared

import "time"

// Types that cross the transactional facade boundary, defined once so every
// backend returns the same shapes to consumers.

type MetricRegistrationEntry struct {
	Name       string
	Unit       string
	MetricType string
}

type ActivePAT struct {
	Id         string
	UserId     int
	Email      string
	LastUsedAt *time.Time
}
