package mcpserver

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
)

// schemaOverrides maps Go types whose custom JSON marshaling diverges from
// their reflected shape. uuid.UUID is a [16]byte array that marshals to a
// string; json.RawMessage is a []byte that marshals to arbitrary JSON (the
// empty schema accepts any value). time.Time already defaults to "string" in
// jsonschema-go, and time.Duration marshals as its underlying int64.
var schemaOverrides = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[uuid.UUID]():       {Type: "string"},
	reflect.TypeFor[json.RawMessage](): {},
}

// outSchema builds the output schema for a tool's result type. The SDK
// validates every result against it at runtime, so a schema that diverges
// from the marshaled shape fails the tool call; the contract suite exercises
// each tool against a real backend to keep that from shipping. Panics on
// inference failure: result types are compile-time constants, so a miss is a
// build defect, not a runtime condition.
func outSchema[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: schemaOverrides})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema for %v: %v", reflect.TypeFor[T](), err))
	}
	allowNullMaps(s)
	return s
}

// allowNullMaps rewrites map-derived schema nodes (type "object" with
// additionalProperties and no properties) to also accept null: jsonschema-go
// marks slices and pointers nullable but not maps, and a nil Go map marshals
// as JSON null.
func allowNullMaps(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if s.Type == "object" && s.Properties == nil && s.AdditionalProperties != nil {
		s.Type = ""
		s.Types = []string{"null", "object"}
	}
	for _, p := range s.Properties {
		allowNullMaps(p)
	}
	allowNullMaps(s.Items)
	allowNullMaps(s.AdditionalProperties)
}
