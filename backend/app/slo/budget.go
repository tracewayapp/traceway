package slo

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

type Window struct {
	Good  int64
	Total int64
}

func (w Window) Bad() int64 {
	return w.Total - w.Good
}

func (w Window) SLI() (float64, bool) {
	if w.Total <= 0 {
		return 0, false
	}
	return float64(w.Good) / float64(w.Total), true
}

func BudgetRemaining(w Window, target float64) (float64, bool) {
	sli, ok := w.SLI()
	if !ok || target >= 1 {
		return 0, false
	}
	return 100 * (1 - (1-sli)/(1-target)), true
}

func BurnRate(w Window, target float64) float64 {
	if w.Total <= 0 || target >= 1 {
		return 0
	}
	return (float64(w.Bad()) / float64(w.Total)) / (1 - target)
}

func Status(w Window, target float64) string {
	sli, ok := w.SLI()
	if !ok {
		return models.SLOStatusNoData
	}
	if sli < target {
		return models.SLOStatusBreached
	}
	return models.SLOStatusOk
}

func Snapshot(window, lastHour, lastSixHours Window, target float64, now time.Time, historyFrom, lastBucketAt *time.Time) models.SLOSnapshot {
	snapshot := models.SLOSnapshot{
		Status:       Status(window, target),
		ComputedAt:   now,
		HistoryFrom:  historyFrom,
		LastBucketAt: lastBucketAt,
	}
	if sli, ok := window.SLI(); ok {
		percent := sli * 100
		snapshot.CurrentSli = &percent
	}
	if remaining, ok := BudgetRemaining(window, target); ok {
		snapshot.BudgetRemaining = &remaining
	}
	if window.Total > 0 {
		burn1h := BurnRate(lastHour, target)
		burn6h := BurnRate(lastSixHours, target)
		snapshot.BurnRate1h = &burn1h
		snapshot.BurnRate6h = &burn6h
	}
	return snapshot
}
