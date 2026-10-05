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
		duration     time.Duration
		lastActivity *time.Time
		inSpan       *time.Time
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
		{name: "second-precision end just before the last segment still closes", endedAt: at(5), lastActivity: at(5.01), now: *at(6), wantEnd: at(5.01), wantDuration: time.Duration(5.01 * float64(time.Minute)), wantRecorded: true},
		{name: "explicit end is measured from the clamped activity", endedAt: at(14), lastActivity: at(-2), now: *at(30), wantEnd: at(14), wantDuration: 14 * time.Minute, wantRecorded: true},
		{name: "segments uploaded after the SDK closed the session keep its close", endedAt: at(60), lastActivity: at(18 * 60), inSpan: at(59.5), now: *at(18*60 + 1), wantEnd: at(60), wantDuration: 60 * time.Minute, wantRecorded: true},
		{name: "a close past the max span falls back to the cap when segments run on", endedAt: at(70), lastActivity: at(18 * 60), inSpan: at(64), now: *at(18*60 + 1), wantEnd: at(65), wantDuration: 65 * time.Minute, wantRecorded: true},
		{name: "a session past the max span is over while segments still arrive", lastActivity: at(300), inSpan: at(40), now: *at(301), wantEnd: at(40), wantDuration: 40 * time.Minute, wantRecorded: true},
		{name: "a tab idle after a few seconds ends there, not at the cap", lastActivity: at(18 * 60), inSpan: at(8.0 / 60), now: *at(18*60 + 1), wantEnd: at(8.0 / 60), wantDuration: 8 * time.Second, wantRecorded: true},
		{name: "segments only past the max span leave nothing recorded inside it", lastActivity: at(300), now: *at(301), wantEnd: at(0), wantDuration: 0, wantRecorded: true},
		{name: "explicit end past the max span is capped", endedAt: at(70), lastActivity: at(64), now: *at(71), wantEnd: at(65), wantDuration: 65 * time.Minute, wantRecorded: true},
		{name: "no segments and an end past the max span is capped", endedAt: at(18 * 60), duration: 18 * time.Hour, now: *at(18*60 + 1), wantEnd: at(65), wantDuration: 65 * time.Minute},
		{name: "a client clock behind the server keeps a live session open", lastActivity: at(29), lastReceived: at(129), now: *at(130), wantDuration: 29 * time.Minute, wantRecorded: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := models.Session{StartedAt: start, EndedAt: tc.endedAt, Duration: tc.duration.Nanoseconds()}
			ResolveSessionEnd(&s, SessionActivity{LastActivity: tc.lastActivity, LastActivityInSpan: tc.inSpan, LastReceived: tc.lastReceived}, tc.now)
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

func TestSegmentsWithinSession(t *testing.T) {
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) time.Time { return start.Add(time.Duration(minutes) * time.Minute) }
	ptr := func(v time.Time) *time.Time { return &v }
	segments := []models.SessionRecording{
		{SegmentIndex: 0, RecordedAt: at(1), EndedAt: ptr(at(1))},
		{SegmentIndex: 1, RecordedAt: at(66), EndedAt: ptr(at(65))},
		{SegmentIndex: 2, RecordedAt: at(64)},
		{SegmentIndex: 3, RecordedAt: at(70), EndedAt: ptr(at(70))},
		{SegmentIndex: 4, RecordedAt: at(900)},
	}
	kept := SegmentsWithinSession(segments, start.Add(SessionMaxSpan))
	if len(kept) != 3 || kept[0].SegmentIndex != 0 || kept[1].SegmentIndex != 1 || kept[2].SegmentIndex != 2 {
		t.Fatalf("kept %+v, want segments 0, 1, 2", kept)
	}
}

func TestResolveSessionEndCutoff(t *testing.T) {
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	at := func(minutes int) *time.Time {
		v := start.Add(time.Duration(minutes) * time.Minute)
		return &v
	}
	cases := []struct {
		name         string
		endedAt      *time.Time
		lastActivity *time.Time
		inSpan       *time.Time
		want         time.Time
	}{
		{name: "no segments", endedAt: at(20), want: *at(65)},
		{name: "activity within the cap", endedAt: at(20), lastActivity: at(30), want: *at(65)},
		{name: "activity past the cap with the SDK close inside it", endedAt: at(60), lastActivity: at(600), inSpan: at(59), want: *at(60)},
		{name: "activity past the cap ends at the last activity inside it", lastActivity: at(600), inSpan: at(12), want: *at(12)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := models.Session{StartedAt: start, EndedAt: tc.endedAt}
			ResolveSessionEnd(&s, SessionActivity{LastActivity: tc.lastActivity, LastActivityInSpan: tc.inSpan}, *at(601))
			if !s.CutoffAt.Equal(tc.want) {
				t.Fatalf("CutoffAt = %v, want %v", s.CutoffAt, tc.want)
			}
		})
	}
}
