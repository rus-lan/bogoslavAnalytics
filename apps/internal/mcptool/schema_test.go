package mcptool_test

import (
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/mcptool"
)

// TestToolInputSchemas_areDraft2020_12Objects is the acceptance check for
// TZ.md sections 7.1 and 10: every tool's inferred input schema is JSON
// Schema draft 2020-12 and has type "object", exactly as mcp.AddTool
// would build it (AddTool infers a nil InputSchema by calling
// jsonschema.ForType(rt, &jsonschema.ForOptions{}) on the In type
// parameter -- see mcp/server.go's toolForErr/setSchema -- the same call
// made directly here). No $schema value is pinned on any of these input
// types, so -- mirroring classify.TestResultSchema_isDraft2020_12's own
// rationale -- an unset Schema field means the library's documented
// default draft, 2020-12, applies.
//
// This test also answers TZ.md section 10's build-time question directly:
// it imports these types from apps/internal/mcptool, a package other than
// its own (package mcptool_test), and calls jsonschema.ForType on them
// exactly as apps/cmd/bogoslav-mcp/server.go's AddTool calls do. A future
// contracts/openapi.yaml generator importing the same types would
// therefore produce byte-identical schemas by construction, not by
// chance.
func TestToolInputSchemas_areDraft2020_12Objects(t *testing.T) {
	cases := []struct {
		tool string
		typ  reflect.Type
	}{
		{"find_mrs", reflect.TypeFor[mcptool.FindMRsInput]()},
		{"get_comments", reflect.TypeFor[mcptool.GetCommentsInput]()},
		{"get_classify_batch", reflect.TypeFor[mcptool.GetClassifyBatchInput]()},
		{"save_labels", reflect.TypeFor[mcptool.SaveLabelsInput]()},
		{"filter_comments", reflect.TypeFor[mcptool.FilterCommentsInput]()},
		{"get_stats", reflect.TypeFor[mcptool.GetStatsInput]()},
	}

	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			schema, err := jsonschema.ForType(tc.typ, &jsonschema.ForOptions{})
			if err != nil {
				t.Fatalf("jsonschema.ForType(%s) error = %v", tc.typ, err)
			}
			if schema.Schema != "" {
				t.Errorf("Schema.Schema = %q, want unset (no draft pinned; defaults to 2020-12)", schema.Schema)
			}
			if schema.Type != "object" {
				t.Errorf("Schema.Type = %q, want %q", schema.Type, "object")
			}
		})
	}
}

// TestFindMRsInput_moreThanIsRequired confirms more_than has no
// omitempty/omitzero tag (TZ.md section 7.2 lists it without "?", unlike
// group/project/mr/strict): jsonschema-go marks a field required exactly
// when it lacks that tag (see jsonschema-go's infer.go), so this is a
// direct, source-grounded check that more_than cannot be silently
// omitted from a tool call.
func TestFindMRsInput_moreThanIsRequired(t *testing.T) {
	schema, err := jsonschema.For[mcptool.FindMRsInput](&jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("jsonschema.For[FindMRsInput]() error = %v", err)
	}
	found := false
	for _, name := range schema.Required {
		if name == "more_than" {
			found = true
		}
	}
	if !found {
		t.Errorf("Required = %v, want it to include %q", schema.Required, "more_than")
	}
}
