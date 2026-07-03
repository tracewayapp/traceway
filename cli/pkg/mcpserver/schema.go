package mcpserver

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
)

var schemaOverrides = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[uuid.UUID]():       {Type: "string"},
	reflect.TypeFor[json.RawMessage](): {},
}

func outSchema[T any]() *jsonschema.Schema {
	s, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: schemaOverrides})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema for %v: %v", reflect.TypeFor[T](), err))
	}
	allowNullMaps(s)
	return s
}

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
