package shared

import (
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

const SessionIdleTimeout = 15 * time.Minute

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

	last := *activity.LastActivity
	if last.Before(s.StartedAt) {
		last = s.StartedAt
	}

	endedByRecord := s.EndedAt != nil && !s.EndedAt.Before(last)
	end := last
	if endedByRecord && s.EndedAt.Before(last.Add(SessionIdleTimeout)) {
		end = *s.EndedAt
	}

	lastReceived := last
	if activity.LastReceived != nil {
		lastReceived = *activity.LastReceived
	}
	idle := now.Sub(lastReceived) >= SessionIdleTimeout

	s.Duration = end.Sub(s.StartedAt).Nanoseconds()
	if endedByRecord || idle {
		s.EndedAt = &end
	} else {
		s.EndedAt = nil
	}
}
