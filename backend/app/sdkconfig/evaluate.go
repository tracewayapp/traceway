package sdkconfig

import (
	"crypto/sha256"
	"encoding/json"
	"time"
)

type Rule struct {
	Id         string
	Name       string
	Conditions Conditions
	Settings   Settings
}

// CaptureActions is what a capture asks its clients to do, as stored with the capture.
type CaptureActions struct {
	LookbackSeconds *float64 `json:"lookbackSeconds,omitempty"`
	Signals         []string `json:"signals,omitempty"`
	Replay          *bool    `json:"replay,omitempty"`
}

type ActiveCapture struct {
	Id              string
	Conditions      Conditions
	ExpiresAt       time.Time
	LookbackSeconds *float64
	Signals         []string
	Replay          *bool
}

type Compiled struct {
	Rules    []Rule
	Captures []ActiveCapture
}

type Result struct {
	Settings        Settings
	MatchedRules    []string
	MatchedCaptures []string
}

// Evaluate builds the document one client receives: every matching rule laid over the previous ones in order,
// then the captures aimed at it. A client nothing matches gets an empty overlay and runs on the SDK's defaults.
func (c *Compiled) Evaluate(attrs map[string]string, instanceUid [16]byte, now time.Time) Result {
	result := Result{Settings: Settings{SchemaVersion: SchemaVersion}}
	if c == nil {
		return result
	}
	for _, rule := range c.Rules {
		if rule.Conditions.Matches(attrs, instanceUid, rule.Id) {
			Merge(&result.Settings, rule.Settings)
			result.MatchedRules = append(result.MatchedRules, rule.Id)
		}
	}
	for _, capture := range c.Captures {
		if !capture.ExpiresAt.After(now) || !capture.Conditions.Matches(attrs, instanceUid, capture.Id) {
			continue
		}
		result.Settings.Captures = append(result.Settings.Captures, Capture{
			Id:              capture.Id,
			Until:           capture.ExpiresAt.UTC().Format(time.RFC3339),
			LookbackSeconds: capture.LookbackSeconds,
			Signals:         capture.Signals,
			Replay:          capture.Replay,
		})
		result.MatchedCaptures = append(result.MatchedCaptures, capture.Id)
	}
	return result
}

// Render serializes a document and hashes it. The hash is the OpAMP config_hash: a client that reports it
// already applied this hash is not sent the document again.
func Render(s Settings) (body []byte, hash []byte, err error) {
	s.SchemaVersion = SchemaVersion
	body, err = json.Marshal(s)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256(body)
	return body, sum[:], nil
}
