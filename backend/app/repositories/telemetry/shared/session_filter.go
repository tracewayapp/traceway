package shared

// SessionAttributeFilter matches a literal attribute key. Excluded filters also
// include sessions where the key is absent; contains matches ignore case.
type SessionAttributeFilter struct {
	Key      string
	Value    string
	Exclude  bool
	Contains bool
}
