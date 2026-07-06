package repositories

import (
	"github.com/tracewayapp/traceway/backend/app/repositories/telemetry/shared"
)

// Aliases and re-exports so consumers keep using the repositories package
// regardless of which telemetry backend is compiled in.
type (
	SessionAttributeFilter = shared.SessionAttributeFilter
	FiredNotification      = shared.FiredNotification
	LogAttributeFilter     = shared.LogAttributeFilter
	LogSearchParams        = shared.LogSearchParams
)

var (
	ComputeImpactReason = shared.ComputeImpactReason
	ComputeImpactScore  = shared.ComputeImpactScore
)
