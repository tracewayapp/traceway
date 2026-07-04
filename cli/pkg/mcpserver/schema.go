package mcpserver

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
)

var schemaOverrides = map[reflect.Type]*jsonschema.Schema{
	reflect.TypeFor[uuid.UUID]():       {Type: "string"},
	reflect.TypeFor[json.RawMessage](): {},
}

// outSchemas memoizes derived schemas per output type: the backend mount
// builds a new server per MCP session, and the schemas are type-static.
// The stable pointers also make the shared mcp.SchemaCache hit by identity.
var outSchemas sync.Map

func outSchema[T any]() *jsonschema.Schema {
	t := reflect.TypeFor[T]()
	if cached, ok := outSchemas.Load(t); ok {
		return cached.(*jsonschema.Schema)
	}
	s, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: schemaOverrides})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: output schema for %v: %v", t, err))
	}
	allowNullMaps(s)
	cached, _ := outSchemas.LoadOrStore(t, s)
	return cached.(*jsonschema.Schema)
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
