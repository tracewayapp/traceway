package output

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// RenderYAML marshals v as YAML to w, applying the same field projection as
// RenderJSON when fields is non-nil. Internally we round-trip through JSON so
// that callers' types only need json struct tags (no need for parallel yaml
// tags on every struct in pkg/client).
func RenderYAML(w io.Writer, v any, fields []string) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return err
	}
	if fields != nil {
		generic = projectFields(generic, fields)
	}
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	// Encode flushes the whole document, so Close only finalizes the stream and
	// has no buffered output left that could fail here.
	defer func() { _ = enc.Close() }()
	return enc.Encode(generic)
}
