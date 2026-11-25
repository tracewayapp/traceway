package shared

// MetricRollupRepairReport is what one pass of the metric rollup repair did.
type MetricRollupRepairReport struct {
	Partitions int
	RowsBefore uint64
	RowsAfter  uint64
	// Partitions left for a later pass, each with the reason.
	Skipped []string
}
