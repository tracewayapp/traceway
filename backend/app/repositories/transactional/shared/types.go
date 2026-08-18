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

type ActiveSetupToken struct {
	Id             string
	UserId         int
	OrganizationId int
	Email          string
	ExpiresAt      time.Time
}

type SetupPlanRow struct {
	Id               string
	UserId           int
	OrganizationId   int
	RequestedByEmail string
	Payload          string
	Status           string
	RejectReason     string
	Result           string
	CreatedAt        time.Time
	DecidedAt        *time.Time
	DecidedBy        *int
}
