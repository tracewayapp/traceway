package shared

import (
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/app/models"
)

func TestResolveSessionEnd(t *testing.T) {
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	at := func(minutes float64) *time.Time {
		v := start.Add(time.Duration(minutes * float64(time.Minute)))
		return &v
	}
	cases := []struct {
		name         string
		endedAt      *time.Time
		lastActivity *time.Time
		lastReceived *time.Time
		now          time.Time
		wantEnd      *time.Time
		wantDuration time.Duration
		wantRecorded bool
	}{
		{name: "no segments keeps the stored end", endedAt: at(3), now: *at(60), wantEnd: at(3), wantDuration: 0},
		{name: "no segments and no end stays open", now: *at(60)},
		{name: "end record lost, idle session ends at last activity", lastActivity: at(4), now: *at(30), wantEnd: at(4), wantDuration: 4 * time.Minute, wantRecorded: true},
		{name: "recent activity is in progress", lastActivity: at(20), now: *at(25), wantDuration: 20 * time.Minute, wantRecorded: true},
		{name: "idle is measured on server receive time", lastActivity: at(4), lastReceived: at(20), now: *at(30), wantDuration: 4 * time.Minute, wantRecorded: true},
		{name: "pagehide end shortly after activity is kept", endedAt: at(5), lastActivity: at(4), now: *at(6), wantEnd: at(5), wantDuration: 5 * time.Minute, wantRecorded: true},
		{name: "late inactivity-timer end falls back to activity", endedAt: at(125), lastActivity: at(10), now: *at(130), wantEnd: at(10), wantDuration: 10 * time.Minute, wantRecorded: true},
		{name: "activity after the end record reopens the session", endedAt: at(20), lastActivity: at(50), now: *at(55), wantDuration: 50 * time.Minute, wantRecorded: true},
		{name: "activity before start on a skewed clock clamps to zero", lastActivity: at(-2), now: *at(30), wantEnd: at(0), wantDuration: 0, wantRecorded: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := models.Session{StartedAt: start, EndedAt: tc.endedAt}
			ResolveSessionEnd(&s, SessionActivity{LastActivity: tc.lastActivity, LastReceived: tc.lastReceived}, tc.now)
			if s.HasRecording != tc.wantRecorded {
				t.Fatalf("HasRecording = %v, want %v", s.HasRecording, tc.wantRecorded)
			}
			if (s.EndedAt == nil) != (tc.wantEnd == nil) || (s.EndedAt != nil && !s.EndedAt.Equal(*tc.wantEnd)) {
				t.Fatalf("EndedAt = %v, want %v", s.EndedAt, tc.wantEnd)
			}
			if time.Duration(s.Duration) != tc.wantDuration {
				t.Fatalf("Duration = %v, want %v", time.Duration(s.Duration), tc.wantDuration)
			}
		})
	}
}
