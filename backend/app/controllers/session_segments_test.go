package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
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

// TW-1: the session recording keeps only the objects of a stored segment's arrays.
func TestDecodeArrayKeepsOnlyObjects(t *testing.T) {
	got := decodeArray(json.RawMessage(`[null,1,"x",{"a":1},[2], {"b":2} ]`))
	if len(got) != 2 || string(got[0]) != `{"a":1}` || string(got[1]) != `{"b":2}` {
		t.Fatalf("decodeArray = %q", got)
	}
	for _, raw := range []string{`{}`, `"abc"`, `7`, `null`, ``, `[`} {
		if got := decodeArray(json.RawMessage(raw)); len(got) != 0 {
			t.Errorf("decodeArray(%s) = %q, want nothing", raw, got)
		}
	}
}

// TW-1: the issue page reads a clip's file through loadSessionRecording; malformed parts must not reach the
// dashboard, and a file that does not parse must not break the response.
func TestLoadSessionRecordingCleansTheFile(t *testing.T) {
	store := newMemStore()
	prev := storage.Store
	storage.Store = store
	t.Cleanup(func() { storage.Store = prev })
	ctx := context.Background()
	_ = store.Write(ctx, "clip.json", []byte(`{"exceptionId":"x","events":[null,{"type":2,"timestamp":1,"data":{"node":{}}},5],"logs":{"level":"error"},"actions":[null,{"type":"custom","timestamp":"2026-11-27T10:00:00Z"}],"startedAt":"2026-11-27T10:00:00Z"}`))
	_ = store.Write(ctx, "legacy.json", []byte(`[{"type":"flutter_video","data":"AA"},null]`))
	_ = store.Write(ctx, "scalar-events.json", []byte(`{"events":"abc"}`))
	_ = store.Write(ctx, "broken.json", []byte(`{"events":[{"type":2`))

	var clip struct {
		Events    []json.RawMessage `json:"events"`
		Logs      json.RawMessage   `json:"logs"`
		Actions   []json.RawMessage `json:"actions"`
		StartedAt *time.Time        `json:"startedAt"`
	}
	raw, err := loadSessionRecording(ctx, "clip.json")
	if err != nil || json.Unmarshal(raw, &clip) != nil {
		t.Fatalf("clip = %s, %v", raw, err)
	}
	if len(clip.Events) != 1 || len(clip.Actions) != 1 || (len(clip.Logs) > 0 && string(clip.Logs) != "null") || clip.StartedAt == nil {
		t.Errorf("clip not cleaned: %s", raw)
	}
	raw, err = loadSessionRecording(ctx, "legacy.json")
	if err != nil || json.Unmarshal(raw, &clip) != nil || len(clip.Events) != 1 {
		t.Errorf("legacy clip = %s, %v", raw, err)
	}
	raw, err = loadSessionRecording(ctx, "scalar-events.json")
	if err != nil || !json.Valid(raw) || bytes.Contains(raw, []byte(`"abc"`)) {
		t.Errorf("events that are not an array must not reach the dashboard: %s, %v", raw, err)
	}
	if raw, err := loadSessionRecording(ctx, "broken.json"); err == nil {
		t.Errorf("a file that does not parse must be reported, not embedded: %s", raw)
	}
}
