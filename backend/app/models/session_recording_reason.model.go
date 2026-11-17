package models

import "time"

type SessionRecordingReason struct {
	TriggerId string    `json:"triggerId"`
	Name      string    `json:"name"`
	Sequence  string    `json:"sequence"`
	Revision  string    `json:"revision"`
	FiredAt   time.Time `json:"firedAt"`
}
