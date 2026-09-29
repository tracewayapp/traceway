package shared

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const SessionIdleTimeout = 15 * time.Minute

const SessionMaxSpan = 65 * time.Minute

const sessionEndTolerance = time.Second

type SessionActivity struct {
	LastActivity *time.Time
	LastReceived *time.Time
}

func SessionRecordingWindow(from, to time.Time) (time.Time, time.Time) {
	lower, _ := TraceWindowBounds(from)
	_, upper := TraceWindowBounds(to)
	return lower, upper
}

func ResolveSessionEnd(s *models.Session, activity SessionActivity, now time.Time) {
	s.HasRecording = activity.LastActivity != nil
	if activity.LastActivity == nil {
		return
	}

	limit := s.StartedAt.Add(SessionMaxSpan)
	last := *activity.LastActivity
	if last.Before(s.StartedAt) {
		last = s.StartedAt
	}
	if last.After(limit) {
		last = limit
	}

	end := last
	if s.EndedAt != nil && s.EndedAt.After(last) && s.EndedAt.Before(last.Add(SessionIdleTimeout)) {
		end = *s.EndedAt
	}
	if end.After(limit) {
		end = limit
	}
	endedByRecord := s.EndedAt != nil && !s.EndedAt.Before(last.Add(-sessionEndTolerance))

	lastReceived := last
	if activity.LastReceived != nil {
		lastReceived = *activity.LastReceived
	}
	idle := now.Sub(lastReceived) >= SessionIdleTimeout || !now.Before(limit)

	s.Duration = end.Sub(s.StartedAt).Nanoseconds()
	if endedByRecord || idle {
		s.EndedAt = &end
	} else {
		s.EndedAt = nil
	}
}

func SegmentsWithinSession(segments []models.SessionRecording, startedAt time.Time) []models.SessionRecording {
	limit := startedAt.Add(SessionMaxSpan)
	kept := segments[:0]
	for _, seg := range segments {
		end := seg.RecordedAt
		if seg.EndedAt != nil {
			end = *seg.EndedAt
		}
		if !end.After(limit) {
			kept = append(kept, seg)
		}
	}
	return kept
}
