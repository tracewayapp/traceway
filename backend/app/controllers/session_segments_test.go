package controllers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
)

func TestUniqueSegmentFilesKeepsOneRowPerFile(t *testing.T) {
	segments := []models.SessionRecording{
		{SegmentIndex: 0, FilePath: "recordings/p/sessions/s/0.json"},
		{SegmentIndex: 1, FilePath: "recordings/p/sessions/s/1.json"},
		{SegmentIndex: 1, FilePath: "recordings/p/sessions/s/1.json"},
		{SegmentIndex: 2, FilePath: "recordings/p/sessions/s/2.json"},
	}
	got := uniqueSegmentFiles(segments)
	if len(got) != 3 || got[0].SegmentIndex != 0 || got[1].SegmentIndex != 1 || got[2].SegmentIndex != 2 {
		t.Fatalf("unexpected segments: %+v", got)
	}
}

func TestTimeVideoEventsStampsEachClipWithItsSegment(t *testing.T) {
	started := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	ended := started.Add(8 * time.Second)
	events := []json.RawMessage{
		json.RawMessage(`{"type":"flutter_video","data":"AAAA","format":"mp4","fps":15,"durationSeconds":8}`),
		json.RawMessage(`{"type":"flutter_video","startedAt":"2026-11-18T09:00:00Z","data":"AAAA"}`),
		json.RawMessage(`{"type":2,"data":{"node":{"type":0}},"timestamp":1}`),
		json.RawMessage(`{"type":3,"data":{"text":"flutter_video"},"timestamp":2}`),
	}
	got := timeVideoEvents(events, &started, &ended)

	var clip struct {
		Type            string    `json:"type"`
		Data            string    `json:"data"`
		DurationSeconds float64   `json:"durationSeconds"`
		StartedAt       time.Time `json:"startedAt"`
		EndedAt         time.Time `json:"endedAt"`
	}
	if err := json.Unmarshal(got[0], &clip); err != nil {
		t.Fatalf("stamped clip %s does not parse: %v", got[0], err)
	}
	if clip.Type != "flutter_video" || clip.Data != "AAAA" || clip.DurationSeconds != 8 || !clip.StartedAt.Equal(started) || !clip.EndedAt.Equal(ended) {
		t.Errorf("stamped clip = %+v", clip)
	}
	if string(got[1]) != `{"type":"flutter_video","startedAt":"2026-11-18T09:00:00Z","data":"AAAA"}` {
		t.Errorf("a clip with its own start was changed: %s", got[1])
	}
	if string(got[2]) != `{"type":2,"data":{"node":{"type":0}},"timestamp":1}` || string(got[3]) != `{"type":3,"data":{"text":"flutter_video"},"timestamp":2}` {
		t.Errorf("rrweb events were changed: %s %s", got[2], got[3])
	}

	untimed := []json.RawMessage{json.RawMessage(`{"type":"flutter_video","data":"AAAA"}`)}
	if got := timeVideoEvents(untimed, nil, nil); string(got[0]) != `{"type":"flutter_video","data":"AAAA"}` {
		t.Errorf("a segment without a start changed its clip: %s", got[0])
	}
}
