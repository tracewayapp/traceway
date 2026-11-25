package telemetry

import (
	"github.com/tracewayapp/traceway/backend/v2/app/repositories/telemetry/shared"
)

// Aliases and re-exports so consumers keep using the telemetry package
// regardless of which telemetry backend is compiled in.
type (
	SessionAttributeFilter   = shared.SessionAttributeFilter
	FiredNotification        = shared.FiredNotification
	CheckResult              = shared.CheckResult
	LogAttributeFilter       = shared.LogAttributeFilter
	LogSearchParams          = shared.LogSearchParams
	LogAttributeTarget       = shared.LogAttributeTarget
	SpanLookup               = shared.SpanLookup
	SLOBucket                = shared.SLOBucket
	Histogram                = shared.Histogram
	MetricHistogramCell      = shared.MetricHistogramCell
	MetricRollupRepairReport = shared.MetricRollupRepairReport
	MetricExemplar           = shared.MetricExemplar
	HistogramSeriesPoint     = shared.HistogramSeriesPoint
	SLOBucketPoint           = shared.SLOBucketPoint
	EndpointSLISpec          = shared.EndpointSLISpec
	TaskSLISpec              = shared.TaskSLISpec
	SpanSLISpec              = shared.SpanSLISpec
	SessionSLISpec           = shared.SessionSLISpec
	OtelSpanSearch           = shared.OtelSpanSearch
	OtelAttributeFilter      = shared.OtelAttributeFilter
)

var (
	ComputeImpactReason = shared.ComputeImpactReason
	ComputeImpactScore  = shared.ComputeImpactScore
)
