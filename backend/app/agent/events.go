package agent

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/tracewayapp/traceway/backend/app/models"
	"github.com/tracewayapp/traceway/backend/app/repositories/transactional"
)

// EventSchemaVersion is stamped into every event payload; the stream is a
// wire format read by the run page and the runner protocol.
const EventSchemaVersion = 1

// Event kinds the control plane writes. Executors append their own
// (assistant_text, tool_call, tool_result, usage, question, result) with the
// same envelope.
const (
	EventCreated   = "created"
	EventStatus    = "status"
	EventMessage   = "message"
	EventReclaimed = "reclaimed"
	EventError     = "error"
	// EventCredential records a credential minted for the run: the run token
	// the agent reads telemetry with, or the git credential the harness
	// clones and pushes with.
	EventCredential = "credential"
)

// AppendEvent adds one event to an attempt's stream inside the caller's
// transaction, wrapping the payload with the schema version.
func AppendEvent(tx *sql.Tx, attemptId uuid.UUID, kind string, payload map[string]any, now time.Time) (*models.AgentAttemptEvent, error) {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["schemaVersion"] = EventSchemaVersion
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return transactional.AgentAttemptEventRepository.Append(tx, attemptId, kind, models.JSONText(encoded), now)
}
