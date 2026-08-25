package shared

const MaxTimeSeriesBuckets = 2000

func TimeSeriesRowLimit(maxGroups int) int {
	return (maxGroups + 1) * MaxTimeSeriesBuckets
}

func GroupCapReached[T any](result map[string][]T, groupKey string, maxGroups int) bool {
	if maxGroups <= 0 {
		return false
	}
	if _, seen := result[groupKey]; seen {
		return false
	}
	if len(result) < maxGroups {
		return false
	}
	result[groupKey] = nil
	return true
}
