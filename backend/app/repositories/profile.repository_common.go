package repositories

import (
	"strings"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func normalizeSeriesPoints(points []models.TimeSeriesPoint, normalize, isGauge bool, intervalMinutes int) {
	if !normalize || isGauge {
		return
	}
	secs := intervalMinutes * 60
	if secs < 1 {
		secs = 1
	}
	for i := range points {
		points[i].Value = points[i].Value / float64(secs)
	}
}

func clampBreakdownLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func breakdownLabelKey(dimension string) (string, bool) {
	const prefix = "label:"
	if !strings.HasPrefix(dimension, prefix) {
		return "", false
	}
	return strings.TrimPrefix(dimension, prefix), true
}
