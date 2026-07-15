package classify

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// TestResultSchema_isDraft2020_12 is the acceptance check for TZ.md
// section 10: the generated schema is draft 2020-12. ResultSchema does
// not pin an explicit $schema value (TZ.md section 10 says the key must
// not be set), and the library's documented behavior is to treat an
// unset $schema as the latest supported draft, 2020-12 (see
// jsonschema.Schema.Resolve's doc comment and the package doc's
// "Inference" section). This test both checks that no $schema value was
// pinned and exercises a draft-2020-12-only keyword the generated
// schema relies on (additionalProperties: false rejecting an unlisted
// property) to confirm the schema resolves and validates under that
// draft.
func TestResultSchema_isDraft2020_12(t *testing.T) {
	schema, err := ResultSchema()
	if err != nil {
		t.Fatalf("ResultSchema() error = %v", err)
	}
	if schema.Schema != "" {
		t.Fatalf("Schema.Schema = %q, want unset (no draft pinned in the document itself)", schema.Schema)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	valid := []map[string]any{{"note_id": float64(1), "label": "bug"}}
	if err := resolved.Validate(valid); err != nil {
		t.Errorf("Validate(valid entry) error = %v, want nil", err)
	}

	invalid := []map[string]any{{"note_id": float64(1), "label": "bug", "extra": "nope"}}
	if err := resolved.Validate(invalid); err == nil {
		t.Error("Validate(entry with unlisted property) error = nil, want rejection")
	}

	wrongType := []map[string]any{{"note_id": "not-an-integer", "label": "bug"}}
	if err := resolved.Validate(wrongType); err == nil {
		t.Error("Validate(non-integer note_id) error = nil, want rejection")
	}
}

// TestResultSchema_marshalJSONRoundTrip is the acceptance check for TZ.md
// section 10: the generated schema round-trips through MarshalJSON. It
// marshals the schema, unmarshals it back into a fresh Schema value, and
// marshals that again; the two JSON documents must describe the same
// schema (compared as decoded JSON values, since JSON object key order
// carries no meaning).
func TestResultSchema_marshalJSONRoundTrip(t *testing.T) {
	schema, err := ResultSchema()
	if err != nil {
		t.Fatalf("ResultSchema() error = %v", err)
	}

	first, err := schema.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var roundTripped jsonschema.Schema
	if err := json.Unmarshal(first, &roundTripped); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	second, err := roundTripped.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(round-tripped) error = %v", err)
	}

	var firstValue, secondValue any
	if err := json.Unmarshal(first, &firstValue); err != nil {
		t.Fatalf("Unmarshal(first) error = %v", err)
	}
	if err := json.Unmarshal(second, &secondValue); err != nil {
		t.Fatalf("Unmarshal(second) error = %v", err)
	}

	if !jsonschema.Equal(firstValue, secondValue) {
		t.Errorf("round trip mismatch:\n first = %s\nsecond = %s", first, second)
	}
}
