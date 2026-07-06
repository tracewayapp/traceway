package shared

// SessionAttributeFilter narrows the sessions list by an exact key=value
// match against the JSON `attributes` blob. Defined here so every telemetry
// backend sees the same type.
type SessionAttributeFilter struct {
	Key   string
	Value string
}
