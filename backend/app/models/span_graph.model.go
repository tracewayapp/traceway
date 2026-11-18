package models

const (
	SpanGraphComplete    = "complete"
	SpanGraphPartial     = "partial"
	SpanGraphUnavailable = "unavailable"

	SpanGraphRowLimit              = "row_limit"
	SpanGraphAttributeSize         = "attribute_size"
	SpanGraphAttributeBudget       = "attribute_budget"
	SpanGraphAttributesUnavailable = "attributes_unavailable"
	SpanGraphReadLimit             = "read_limit"
	// SpanGraphMostImportant accompanies row_limit on a whole trace read: the spans kept are errors, entry points and the slowest, not the earliest.
	SpanGraphMostImportant = "most_important"
)

// SpanGraphStatus tells a reader whether the spans beside it are the whole graph.
type SpanGraphStatus struct {
	State             string   `json:"state"`
	Reasons           []string `json:"reasons,omitempty"`
	OmittedAttributes int      `json:"omittedAttributes,omitempty"`
}
