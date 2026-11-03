// Package sdkconfig holds the remote collection settings the OpenTelemetry browser SDK receives over OpAMP: the
// settings document, the conditions that target a rule at a cohort of clients, and the evaluation that turns a
// project's rules into the document one client gets. It has no database or HTTP dependencies.
package sdkconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

// Mode says what the SDK does with a signal: drop it, keep it on the device until a trigger or capture flushes
// it, or send it.
type Mode string

const (
	ModeOff    Mode = "off"
	ModeBuffer Mode = "buffer"
	ModeStream Mode = "stream"
)

const (
	SchemaVersion       = 1
	MaxDocumentBytes    = 16 << 10
	MaxTriggers         = 20
	MaxTriggerSteps     = 5
	MaxTriggerActions   = 5
	MaxMatcherDepth     = 8
	MaxMatcherNodes     = 50
	MaxStepCount        = 1000
	MaxBufferBytes      = 64 << 20
	MaxDurationSeconds  = 24 * 60 * 60
	maxConditionValues  = 100
	maxConditionKeyLen  = 128
	maxConditionTextLen = 256
)

var (
	severities  = map[string]bool{"trace": true, "debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	signalNames = map[string]bool{"errors": true, "messages": true, "network": true, "console": true, "navigation": true,
		"userActions": true, "webVitals": true, "resourceTiming": true, "custom": true, "*": true}
	triggerIdRe = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)
	matcherOps  = map[string]bool{"eq": true, "neq": true, "in": true, "not_in": true, "contains": true, "icontains": true,
		"starts_with": true, "glob": true, "exists": true, "not_exists": true, "gt": true, "gte": true, "lt": true, "lte": true,
		"version": true}
)

// ValidationError is a problem with a document a user wrote; controllers answer it with 422.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

type SignalSettings struct {
	Mode        *Mode    `json:"mode,omitempty"`
	SampleRate  *float64 `json:"sampleRate,omitempty"`
	MinSeverity *string  `json:"minSeverity,omitempty"`
}

type Signals struct {
	Errors         *SignalSettings `json:"errors,omitempty"`
	Messages       *SignalSettings `json:"messages,omitempty"`
	Network        *SignalSettings `json:"network,omitempty"`
	Console        *SignalSettings `json:"console,omitempty"`
	Navigation     *SignalSettings `json:"navigation,omitempty"`
	UserActions    *SignalSettings `json:"userActions,omitempty"`
	WebVitals      *SignalSettings `json:"webVitals,omitempty"`
	ResourceTiming *SignalSettings `json:"resourceTiming,omitempty"`
	Custom         *SignalSettings `json:"custom,omitempty"`
}

func (s *Signals) each(fn func(name string, field **SignalSettings)) {
	fn("errors", &s.Errors)
	fn("messages", &s.Messages)
	fn("network", &s.Network)
	fn("console", &s.Console)
	fn("navigation", &s.Navigation)
	fn("userActions", &s.UserActions)
	fn("webVitals", &s.WebVitals)
	fn("resourceTiming", &s.ResourceTiming)
	fn("custom", &s.Custom)
}

type SessionSettings struct {
	Enabled            *bool    `json:"enabled,omitempty"`
	BaselineSampleRate *float64 `json:"baselineSampleRate,omitempty"`
}

type ReplaySettings struct {
	Mode              *Mode    `json:"mode,omitempty"`
	SessionSampleRate *float64 `json:"sessionSampleRate,omitempty"`
	MaskAllInputs     *bool    `json:"maskAllInputs,omitempty"`
}

type BufferSettings struct {
	MaxBytes       *int64 `json:"maxBytes,omitempty"`
	MaxAgeSeconds  *int64 `json:"maxAgeSeconds,omitempty"`
	ReplayMaxBytes *int64 `json:"replayMaxBytes,omitempty"`
	Persist        *bool  `json:"persist,omitempty"`
}

// Matcher is a condition (Key, Op and Value or Values) or exactly one of All, Any and Not.
type Matcher struct {
	Key    string     `json:"key,omitempty"`
	Op     string     `json:"op,omitempty"`
	Value  any        `json:"value,omitempty"`
	Values []any      `json:"values,omitempty"`
	All    []*Matcher `json:"all,omitempty"`
	Any    []*Matcher `json:"any,omitempty"`
	Not    *Matcher   `json:"not,omitempty"`
}

type TriggerStep struct {
	Match         *Matcher `json:"match"`
	WithinSeconds *float64 `json:"withinSeconds,omitempty"`
	// Count is how many matching records the step needs, e.g. five error lines; PeriodSeconds bounds them to a
	// sliding window.
	Count         *int     `json:"count,omitempty"`
	PeriodSeconds *float64 `json:"periodSeconds,omitempty"`
}

type TriggerAction struct {
	Type            string            `json:"type"`
	LookbackSeconds *float64          `json:"lookbackSeconds,omitempty"`
	DurationSeconds *float64          `json:"durationSeconds,omitempty"`
	Signals         []string          `json:"signals,omitempty"`
	Replay          *bool             `json:"replay,omitempty"`
	EventName       string            `json:"eventName,omitempty"`
	Attributes      map[string]string `json:"attributes,omitempty"`
}

// Trigger runs on the device. In a rule it may be a patch of a trigger an earlier rule or the SDK defines: a
// trigger without steps or actions changes only the fields it sets, so {"id":"default-exception","enabled":false}
// switches the SDK's built-in error clip off.
type Trigger struct {
	Id              string          `json:"id"`
	Name            *string         `json:"name,omitempty"`
	Enabled         *bool           `json:"enabled,omitempty"`
	SampleRate      *float64        `json:"sampleRate,omitempty"`
	CooldownSeconds *float64        `json:"cooldownSeconds,omitempty"`
	MaxPerSession   *int            `json:"maxPerSession,omitempty"`
	Steps           []TriggerStep   `json:"steps,omitempty"`
	Actions         []TriggerAction `json:"actions,omitempty"`
}

// Capture is an on-demand capture from the dashboard. Rules cannot contain one; Evaluate adds the active ones.
type Capture struct {
	Id              string   `json:"id"`
	Until           string   `json:"until"`
	LookbackSeconds *float64 `json:"lookbackSeconds,omitempty"`
	Signals         []string `json:"signals,omitempty"`
	Replay          *bool    `json:"replay,omitempty"`
}

// Limits are what the organization's plan accepts at ingest. The SDK holds every signal at or below them,
// including while a trigger or capture streams, so it never uploads what would be dropped. Only ApplyPermission
// writes them; a rule cannot.
type Limits struct {
	Signals map[string]Mode `json:"signals,omitempty"`
	Replay  *Mode           `json:"replay,omitempty"`
}

// Settings is a partial settings document: nil fields inherit from earlier rules and finally from the SDK's
// built-in defaults. Field order is fixed, so marshalling the same settings always yields the same bytes and hash.
type Settings struct {
	SchemaVersion int              `json:"schemaVersion"`
	Enabled       *bool            `json:"enabled,omitempty"`
	Signals       *Signals         `json:"signals,omitempty"`
	Sessions      *SessionSettings `json:"sessions,omitempty"`
	Replay        *ReplaySettings  `json:"replay,omitempty"`
	Buffer        *BufferSettings  `json:"buffer,omitempty"`
	Triggers      []Trigger        `json:"triggers,omitempty"`
	Captures      []Capture        `json:"captures,omitempty"`
	Limits        *Limits          `json:"limits,omitempty"`
}

// ParseSettings parses and validates the settings part of a rule.
func ParseSettings(raw []byte) (Settings, error) {
	var s Settings
	if len(raw) > MaxDocumentBytes {
		return s, invalid("settings must be at most %d bytes", MaxDocumentBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return Settings{SchemaVersion: SchemaVersion}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&s); err != nil {
		return s, invalid("settings are not a valid document: %v", err)
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}
	return s, s.Validate()
}

func validMode(m *Mode) bool {
	return m == nil || *m == ModeOff || *m == ModeBuffer || *m == ModeStream
}

func validRate(r *float64) bool {
	return r == nil || (*r >= 0 && *r <= 1)
}

func validDuration(d *float64) bool {
	return d != nil && *d > 0 && *d <= MaxDurationSeconds
}

func (s Settings) Validate() error {
	if s.SchemaVersion != SchemaVersion {
		return invalid("schemaVersion must be %d", SchemaVersion)
	}
	if s.Limits != nil {
		return invalid("limits come from the organization's plan and cannot be part of a rule")
	}
	if len(s.Captures) > 0 {
		return invalid("captures are created from the dashboard and cannot be part of a rule")
	}
	if s.Signals != nil {
		var err error
		s.Signals.each(func(name string, field **SignalSettings) {
			if err != nil || *field == nil {
				return
			}
			sig := *field
			switch {
			case !validMode(sig.Mode):
				err = invalid("signals.%s.mode must be off, buffer or stream", name)
			case !validRate(sig.SampleRate):
				err = invalid("signals.%s.sampleRate must be between 0 and 1", name)
			case sig.MinSeverity != nil && !severities[*sig.MinSeverity]:
				err = invalid("signals.%s.minSeverity must be one of trace, debug, info, warn, error, fatal", name)
			}
		})
		if err != nil {
			return err
		}
	}
	if s.Sessions != nil && !validRate(s.Sessions.BaselineSampleRate) {
		return invalid("sessions.baselineSampleRate must be between 0 and 1")
	}
	if s.Replay != nil {
		if !validMode(s.Replay.Mode) {
			return invalid("replay.mode must be off, buffer or stream")
		}
		if !validRate(s.Replay.SessionSampleRate) {
			return invalid("replay.sessionSampleRate must be between 0 and 1")
		}
	}
	if b := s.Buffer; b != nil {
		for name, v := range map[string]*int64{"maxBytes": b.MaxBytes, "replayMaxBytes": b.ReplayMaxBytes} {
			if v != nil && (*v <= 0 || *v > MaxBufferBytes) {
				return invalid("buffer.%s must be between 1 and %d", name, MaxBufferBytes)
			}
		}
		if b.MaxAgeSeconds != nil && (*b.MaxAgeSeconds <= 0 || *b.MaxAgeSeconds > MaxDurationSeconds) {
			return invalid("buffer.maxAgeSeconds must be between 1 and %d", MaxDurationSeconds)
		}
	}
	if len(s.Triggers) > MaxTriggers {
		return invalid("a rule may define at most %d triggers", MaxTriggers)
	}
	seen := map[string]bool{}
	for i, t := range s.Triggers {
		if err := t.validate(); err != nil {
			return invalid("triggers[%d]: %s", i, err.(*ValidationError).Message)
		}
		if seen[t.Id] {
			return invalid("triggers[%d]: duplicate id %q", i, t.Id)
		}
		seen[t.Id] = true
	}
	return nil
}

func (t Trigger) validate() error {
	if !triggerIdRe.MatchString(t.Id) {
		return invalid("id must be 1-64 letters, digits, '.', '_' or '-'")
	}
	if !validRate(t.SampleRate) {
		return invalid("sampleRate must be between 0 and 1")
	}
	if t.CooldownSeconds != nil && (*t.CooldownSeconds < 0 || *t.CooldownSeconds > MaxDurationSeconds) {
		return invalid("cooldownSeconds must be between 0 and %d", MaxDurationSeconds)
	}
	if t.MaxPerSession != nil && *t.MaxPerSession < 0 {
		return invalid("maxPerSession must not be negative")
	}
	if len(t.Steps) > MaxTriggerSteps {
		return invalid("at most %d steps", MaxTriggerSteps)
	}
	for i, step := range t.Steps {
		if step.Match == nil {
			return invalid("steps[%d].match is required", i)
		}
		nodes := 0
		if err := step.Match.validate(0, &nodes); err != nil {
			return invalid("steps[%d].match: %s", i, err.(*ValidationError).Message)
		}
		if step.WithinSeconds != nil && !validDuration(step.WithinSeconds) {
			return invalid("steps[%d].withinSeconds must be between 1 and %d", i, MaxDurationSeconds)
		}
		if step.Count != nil && (*step.Count < 1 || *step.Count > MaxStepCount) {
			return invalid("steps[%d].count must be between 1 and %d", i, MaxStepCount)
		}
		if step.PeriodSeconds != nil && (step.Count == nil || !validDuration(step.PeriodSeconds)) {
			return invalid("steps[%d].periodSeconds needs a count and must be between 1 and %d", i, MaxDurationSeconds)
		}
	}
	if len(t.Actions) > MaxTriggerActions {
		return invalid("at most %d actions", MaxTriggerActions)
	}
	if (len(t.Steps) == 0) != (len(t.Actions) == 0) {
		return invalid("steps and actions are given together, or neither to patch an existing trigger")
	}
	for i, a := range t.Actions {
		if err := a.validate(); err != nil {
			return invalid("actions[%d]: %s", i, err.(*ValidationError).Message)
		}
	}
	return nil
}

func validSignals(signals []string) bool {
	for _, s := range signals {
		if !signalNames[s] {
			return false
		}
	}
	return true
}

func (a TriggerAction) validate() error {
	if !validSignals(a.Signals) {
		return invalid("signals may name errors, messages, network, console, navigation, userActions, webVitals, resourceTiming, custom or *")
	}
	switch a.Type {
	case "flush":
		if !validDuration(a.LookbackSeconds) {
			return invalid("flush needs lookbackSeconds between 1 and %d", MaxDurationSeconds)
		}
	case "stream", "startReplay":
		if !validDuration(a.DurationSeconds) {
			return invalid("%s needs durationSeconds between 1 and %d", a.Type, MaxDurationSeconds)
		}
	case "capture":
		if !validDuration(a.DurationSeconds) {
			return invalid("capture needs durationSeconds between 1 and %d", MaxDurationSeconds)
		}
		if a.LookbackSeconds != nil && !validDuration(a.LookbackSeconds) {
			return invalid("capture lookbackSeconds must be between 1 and %d", MaxDurationSeconds)
		}
	case "emit":
		if a.EventName == "" || len(a.EventName) > maxConditionKeyLen {
			return invalid("emit needs an eventName of at most %d characters", maxConditionKeyLen)
		}
	default:
		return invalid("type must be flush, stream, startReplay, capture or emit")
	}
	return nil
}

func (m *Matcher) validate(depth int, nodes *int) error {
	*nodes++
	if depth > MaxMatcherDepth || *nodes > MaxMatcherNodes {
		return invalid("matcher is nested too deeply or has too many conditions")
	}
	forms := 0
	if m.Key != "" || m.Op != "" {
		forms++
	}
	for _, set := range [][]*Matcher{m.All, m.Any} {
		if set != nil {
			forms++
		}
	}
	if m.Not != nil {
		forms++
	}
	if forms != 1 {
		return invalid("a matcher is a condition (key, op) or exactly one of all, any, not")
	}
	for _, set := range [][]*Matcher{m.All, m.Any} {
		for _, child := range set {
			if child == nil {
				return invalid("matcher lists cannot contain null")
			}
			if err := child.validate(depth+1, nodes); err != nil {
				return err
			}
		}
	}
	if m.Not != nil {
		return m.Not.validate(depth+1, nodes)
	}
	if m.Key == "" {
		return nil
	}
	return validateCondition(m.Key, m.Op, m.Value, m.Values)
}

func validateCondition(key, op string, value any, values []any) error {
	if len(key) > maxConditionKeyLen {
		return invalid("key must be at most %d characters", maxConditionKeyLen)
	}
	if !matcherOps[op] {
		return invalid("op %q is not one of eq, neq, in, not_in, contains, icontains, starts_with, glob, exists, not_exists, gt, gte, lt, lte, version", op)
	}
	if len(values) > maxConditionValues {
		return invalid("at most %d values", maxConditionValues)
	}
	for _, v := range append([]any{value}, values...) {
		switch val := v.(type) {
		case nil, bool, float64:
		case string:
			if len(val) > maxConditionTextLen {
				return invalid("values must be at most %d characters", maxConditionTextLen)
			}
		default:
			return invalid("values must be strings, numbers or booleans")
		}
	}
	switch op {
	case "in", "not_in":
		if len(values) == 0 {
			return invalid("%s needs values", op)
		}
	case "exists", "not_exists":
	default:
		if value == nil {
			return invalid("%s needs a value", op)
		}
	}
	return nil
}
