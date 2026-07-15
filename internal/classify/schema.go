package classify

import "github.com/google/jsonschema-go/jsonschema"

// NoteLabel is one entry in a labeling result: the label the calling
// agent assigned to a single note from the batch (TZ.md section 8.5).
type NoteLabel struct {
	NoteID int64  `json:"note_id" jsonschema:"id of a note from the labeling batch"`
	Label  string `json:"label" jsonschema:"taxonomy label assigned to the note"`
}

// ResultSchema returns the JSON Schema that a labeling result must
// satisfy on the wire: a JSON array of {note_id, label} objects (TZ.md
// section 8.5). It is generated from the NoteLabel Go type by
// github.com/google/jsonschema-go, the same library and code path
// mcp.AddTool uses to infer a tool's inputSchema by reflection (TZ.md
// section 10), so a future get_classify_batch tool and
// contracts/openapi.yaml stay identical by construction. The emitted
// draft is 2020-12 (the library's documented default when no
// $schema is set).
//
// This schema only checks shape: every entry has an integer note_id and
// a string label, and no other properties. The taxonomy- and batch-
// specific checks — the label is a real taxonomy member, every batch
// note_id is labeled exactly once — depend on data a static schema
// cannot carry, so they are enforced by Validate instead.
func ResultSchema() (*jsonschema.Schema, error) {
	return jsonschema.For[[]NoteLabel](&jsonschema.ForOptions{})
}
