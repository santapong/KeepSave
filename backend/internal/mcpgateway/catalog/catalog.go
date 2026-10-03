// Package catalog defines the reviewed installation-qualified tool identities.
// It has no authority, transport or provider dependency.
package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/jsonschema-go/jsonschema"
)

const Installation = "keepsave_v1"
const MaxEnvelopeBytes = 4 << 20

type Definition struct {
	Name         string             `json:"name"`
	Operation    string             `json:"operation"`
	Installation string             `json:"installation"`
	SchemaDigest string             `json:"schema_digest"`
	InputSchema  *jsonschema.Schema `json:"input_schema"`
}

func text(max int) any { return map[string]any{"type": "string", "minLength": 1, "maxLength": max} }
func Definitions() []Definition {
	specs := []struct {
		name       string
		properties map[string]any
		required   []string
	}{
		{"available_runs", map[string]any{}, []string{}},
		{"repository_tree", map[string]any{"run_id": text(36), "request_key": text(128)}, []string{"run_id", "request_key"}},
		{"read_file", map[string]any{"run_id": text(36), "request_key": text(128), "path": text(512)}, []string{"run_id", "request_key", "path"}},
		{"operation_status", map[string]any{"operation_id": text(36), "wait_ms": map[string]any{"type": "integer", "minimum": 0, "maximum": 2000}}, []string{"operation_id"}},
		{"operation_result", map[string]any{"operation_id": text(36)}, []string{"operation_id"}},
		{"cancel_operation", map[string]any{"operation_id": text(36)}, []string{"operation_id"}},
		{"cancel_run", map[string]any{"run_id": text(36)}, []string{"run_id"}},
	}
	out := make([]Definition, 0, len(specs))
	for _, spec := range specs {
		raw, _ := json.Marshal(map[string]any{"type": "object", "properties": spec.properties, "required": spec.required, "additionalProperties": false})
		schema := new(jsonschema.Schema)
		if e := json.Unmarshal(raw, schema); e != nil {
			panic(e)
		}
		normalized, _ := json.Marshal(schema)
		h := sha256.Sum256(normalized)
		out = append(out, Definition{Name: Installation + "__" + spec.name, Operation: spec.name, Installation: Installation, SchemaDigest: hex.EncodeToString(h[:]), InputSchema: schema})
	}
	return out
}
func Find(operation string) Definition {
	for _, d := range Definitions() {
		if d.Operation == operation {
			return d
		}
	}
	panic("unknown reviewed tool operation")
}
