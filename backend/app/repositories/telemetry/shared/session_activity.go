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
	limit := s.StartedAt.Add(SessionMaxSpan)
	s.CutoffAt = limit
	if activity.LastActivity == nil {
		if s.EndedAt != nil && s.EndedAt.After(limit) {
			s.EndedAt = &limit
		}
		s.Duration = min(s.Duration, SessionMaxSpan.Nanoseconds())
		return
	}

	last := *activity.LastActivity
	if last.Before(s.StartedAt) {
		last = s.StartedAt
	}
	pastLimit := last.After(limit)
	if pastLimit {
		last = limit
		if s.EndedAt != nil && s.EndedAt.After(s.StartedAt) && !s.EndedAt.After(limit) {
			last = *s.EndedAt
			s.CutoffAt = last
		}
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
	idle := now.Sub(lastReceived) >= SessionIdleTimeout || pastLimit

	s.Duration = end.Sub(s.StartedAt).Nanoseconds()
	if endedByRecord || idle {
		s.EndedAt = &end
	} else {
		s.EndedAt = nil
	}
}

func SegmentsWithinSession(segments []models.SessionRecording, cutoff time.Time) []models.SessionRecording {
	kept := segments[:0]
	for _, seg := range segments {
		end := seg.RecordedAt
		if seg.EndedAt != nil {
			end = *seg.EndedAt
		}
		if !end.After(cutoff) {
			kept = append(kept, seg)
		}
	}
	return kept
}
