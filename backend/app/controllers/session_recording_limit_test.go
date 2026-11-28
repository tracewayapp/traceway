package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/storage"
)

// TW-5: one read of a session's recording holds every segment in memory several times over, so it stops at a byte
// and a segment cap, at a segment boundary, and says the recording was cut.
func TestSessionRecordingStopsAtItsCaps(t *testing.T) {
	store := newMemStore()
	prevStore, prevBytes, prevSegments := storage.Store, maxSessionRecordingBytes, maxSessionRecordingSegments
	storage.Store = store
	maxSessionRecordingBytes = 2 << 20
	t.Cleanup(func() {
		storage.Store, maxSessionRecordingBytes, maxSessionRecordingSegments = prevStore, prevBytes, prevSegments
	})
	ctx := context.Background()

	padding := strings.Repeat("x", 1000)
	var segments []models.SessionRecording
	for i := range 40 {
		events := make([]string, 128)
		for j := range events {
			events[j] = fmt.Sprintf(`{"type":3,"timestamp":%d,"data":{"source":5,"text":%q}}`, i*1000+j, padding)
		}
		key := fmt.Sprintf("recordings/p/sessions/s/%d.json", i)
		_ = store.Write(ctx, key, []byte(`{"events":[`+strings.Join(events, ",")+`]}`))
		segments = append(segments, models.SessionRecording{SegmentIndex: int32(i), FilePath: key})
	}

	payload := buildSessionRecording(ctx, segments)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !payload.Truncated {
		t.Errorf("a recording past the byte cap must say it was cut")
	}
	if len(body) > maxSessionRecordingBytes+64<<10 {
		t.Errorf("response holds %d bytes, cap %d", len(body), maxSessionRecordingBytes)
	}
	if len(payload.Events) == 0 || len(payload.Events)%128 != 0 {
		t.Fatalf("want whole segments, got %d events", len(payload.Events))
	}
	var first, last struct{ Timestamp int }
	_ = json.Unmarshal(payload.Events[0], &first)
	_ = json.Unmarshal(payload.Events[len(payload.Events)-1], &last)
	if first.Timestamp != 0 || last.Timestamp != (len(payload.Events)/128-1)*1000+127 {
		t.Errorf("want the leading segments in order, got timestamps %d..%d", first.Timestamp, last.Timestamp)
	}

	maxSessionRecordingBytes = prevBytes
	maxSessionRecordingSegments = 10
	payload = buildSessionRecording(ctx, segments)
	if !payload.Truncated || len(payload.Events) != 10*128 {
		t.Errorf("segment cap: truncated=%v events=%d, want the first 10 segments", payload.Truncated, len(payload.Events))
	}

	maxSessionRecordingSegments = prevSegments
	payload = buildSessionRecording(ctx, segments[:3])
	if payload.Truncated || len(payload.Events) != 3*128 {
		t.Errorf("a recording under the caps is whole: truncated=%v events=%d", payload.Truncated, len(payload.Events))
	}
}

// Crash safety: a session's stored segments hold whatever was uploaded before ingest checked it; reading them never
// panics, leaves out what does not parse and answers JSON the response can encode.
func TestSessionRecordingSurvivesMalformedSegments(t *testing.T) {
	store := newMemStore()
	prev := storage.Store
	storage.Store = store
	t.Cleanup(func() { storage.Store = prev })
	ctx := context.Background()
	started := `"startedAt":"2026-11-27T10:00:00Z","endedAt":"2026-11-27T10:00:08Z"`
	files := []string{
		`{"events":[{"type":4,"timestamp":1,"data":{}},null,7,"flutter_video",[1]],"logs":{"level":"error"},"actions":"x"}`,
		`{"events":[{"type":"flutter_video"},{"type":"flutter_video","startedAt":5},{"type":"flutter_video","data":"AA"}],` + started + `}`,
		`[{"type":2}]`,
		`{"events":[{"type":2`,
		`{"startedAt":"yesterday","events":[{"type":2}]}`,
		`null`,
		``,
		`{"events":null,"logs":null,"actions":null}`,
	}
	var segments []models.SessionRecording
	for i, file := range files {
		key := fmt.Sprintf("recordings/p/sessions/s/%d.json", i)
		_ = store.Write(ctx, key, []byte(file))
		segments = append(segments, models.SessionRecording{SegmentIndex: int32(i), FilePath: key})
	}
	segments = append(segments, models.SessionRecording{SegmentIndex: 99, FilePath: "missing.json"})

	payload := buildSessionRecording(ctx, segments)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("payload does not encode: %v", err)
	}
	if len(payload.Events) != 4 || len(payload.Logs) != 0 || len(payload.Actions) != 0 {
		t.Errorf("payload = %s", body)
	}
	for _, event := range payload.Events {
		if !json.Valid(event) || event[0] != '{' {
			t.Errorf("event %s is no object", event)
		}
	}
}
