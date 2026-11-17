package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/v2/app/models"
	"github.com/tracewayapp/traceway/backend/v2/app/sdkconfig"
)

type reasonRows interface {
	Next() bool
	Scan(...any) error
	Close() error
	Err() error
}

// The firing record is diagnostic client telemetry, not proof that replay bytes arrived.
// Keep its immutable sequence snapshot separate from mutable session attributes.
func StoreSessionRecordingReasons(ctx context.Context, records []models.LogRecord) error {
	for _, record := range records {
		a := record.LogAttributes
		if a["event.name"] != "traceway.trigger.fired" || a["traceway.trigger.replay"] != "true" {
			continue
		}
		session := a["session.id"]
		if session == "" {
			session = record.ResourceAttributes["session.id"]
		}
		sessionId, err := uuid.Parse(session)
		if err != nil || sessionId == uuid.Nil {
			continue
		}
		raw := a["traceway.trigger.steps"]
		if len(raw) > 16384 {
			continue
		}
		var steps []sdkconfig.TriggerStep
		if json.Unmarshal([]byte(raw), &steps) != nil || len(steps) == 0 {
			continue
		}
		duration := 1.0
		trigger := sdkconfig.Trigger{Id: a["traceway.trigger.id"], Steps: steps, Actions: []sdkconfig.TriggerAction{{Type: "startReplay", DurationSeconds: &duration}}}
		if (sdkconfig.Settings{SchemaVersion: 1, Triggers: []sdkconfig.Trigger{trigger}}).Validate() != nil {
			continue
		}
		parts := make([]string, 0, len(steps))
		for _, step := range steps {
			if step.Match.Event != nil {
				e := step.Match.Event
				parts = append(parts, fmt.Sprintf("%s(%s)", e.Type, e.Value))
			} else {
				parts = append(parts, "event")
			}
		}
		canonical, _ := json.Marshal(steps)
		hash := sha256.Sum256(append([]byte(a["traceway.trigger.name"]+"\x00"), canonical...))
		name := a["traceway.trigger.name"]
		if len(name) > 128 {
			name = string([]rune(name)[:min(128, len([]rune(name)))])
		}
		if name == "" {
			name = trigger.Id
		}
		at := record.Timestamp
		if at.Before(time.Now().Add(-72*time.Hour)) || at.After(time.Now().Add(5*time.Minute)) {
			continue
		}
		if err := reasonInsert(ctx, record.ProjectId, sessionId, trigger.Id, hex.EncodeToString(hash[:]), name, strings.Join(parts, " → "), at.UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}

func LoadSessionRecordingReasons(ctx context.Context, project uuid.UUID, sessions []models.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	placeholders := make([]string, len(sessions))
	args := []any{project}
	index := map[uuid.UUID]int{}
	for i, s := range sessions {
		placeholders[i] = "?"
		args = append(args, s.Id)
		index[s.Id] = i
	}
	rows, err := reasonQuery(ctx, "SELECT session_id, trigger_id, revision, name, sequence, MIN(fired_at) FROM session_recording_reasons WHERE project_id = ? AND session_id IN ("+strings.Join(placeholders, ",")+") GROUP BY session_id, trigger_id, revision, name, sequence ORDER BY MIN(fired_at) LIMIT 2000", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var reason models.SessionRecordingReason
		var at int64
		if err := rows.Scan(&id, &reason.TriggerId, &reason.Revision, &reason.Name, &reason.Sequence, &at); err != nil {
			return err
		}
		reason.FiredAt = time.UnixMilli(at).UTC()
		if i, ok := index[id]; ok {
			sessions[i].RecordingReasons = append(sessions[i].RecordingReasons, reason)
		}
	}
	return rows.Err()
}

func ListSessionRecordingReasons(ctx context.Context, project uuid.UUID) ([]models.SessionRecordingReason, error) {
	rows, err := reasonQuery(ctx, "SELECT trigger_id, name, sequence, revision, MAX(fired_at) FROM session_recording_reasons WHERE project_id = ? GROUP BY trigger_id, name, sequence, revision ORDER BY MAX(fired_at) DESC LIMIT 200", project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.SessionRecordingReason{}
	for rows.Next() {
		var reason models.SessionRecordingReason
		var at int64
		if err := rows.Scan(&reason.TriggerId, &reason.Name, &reason.Sequence, &reason.Revision, &at); err != nil {
			return nil, err
		}
		reason.FiredAt = time.UnixMilli(at).UTC()
		result = append(result, reason)
	}
	return result, rows.Err()
}
