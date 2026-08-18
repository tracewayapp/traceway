package transactional

import "github.com/tracewayapp/traceway/backend/app/repositories/transactional/shared"

// Aliases so consumers keep using the transactional package regardless of
// which relational backend is compiled in.
type (
	ActivePAT               = shared.ActivePAT
	ActiveSetupToken        = shared.ActiveSetupToken
	MetricRegistrationEntry = shared.MetricRegistrationEntry
	SetupPlanRow            = shared.SetupPlanRow
)
