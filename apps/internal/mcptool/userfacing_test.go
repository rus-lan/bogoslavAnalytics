package mcptool_test

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/mcptool"
)

// sectionRefPattern catches "section 7", "section 9.3.2", etc., case
// insensitively, wherever it turns up in a jsonschema tag.
var sectionRefPattern = regexp.MustCompile(`(?i)\bsection\s+\d`)

// collectJSONSchemaTagDescriptions returns every "jsonschema" struct tag
// value declared directly on typ's fields: the exact text an agent reads
// as a property's description over tools/list.
func collectJSONSchemaTagDescriptions(typ reflect.Type) []string {
	var out []string
	for i := 0; i < typ.NumField(); i++ {
		if tag, ok := typ.Field(i).Tag.Lookup("jsonschema"); ok {
			out = append(out, tag)
		}
	}
	return out
}

// toolTypes lists every type mcptool declares whose jsonschema tags become
// MCP tool documentation: the six tools' input types, and every declared
// output type.
var toolTypes = []reflect.Type{
	reflect.TypeFor[mcptool.FindMRsInput](),
	reflect.TypeFor[mcptool.FindMRsOutput](),
	reflect.TypeFor[mcptool.GetCommentsInput](),
	reflect.TypeFor[mcptool.GetCommentsOutput](),
	reflect.TypeFor[mcptool.GetClassifyBatchInput](),
	reflect.TypeFor[mcptool.GetClassifyBatchOutput](),
	reflect.TypeFor[mcptool.SaveLabelsInput](),
	reflect.TypeFor[mcptool.SaveLabelsOutput](),
	reflect.TypeFor[mcptool.FilterCommentsInput](),
	reflect.TypeFor[mcptool.FilterCommentsOutput](),
	reflect.TypeFor[mcptool.GetStatsInput](),
	reflect.TypeFor[mcptool.GetStatsOutput](),
}

// TestToolTypes_jsonschemaTagsHaveNoDanglingInternalReferences proves that
// the descriptions bogoslav-mcp publishes to a calling agent over
// tools/list never point that agent at TZ.md, a TZ.md section number, or
// an internal/ package path: none of those resolve for an agent that only
// ever sees this tool's schema.
func TestToolTypes_jsonschemaTagsHaveNoDanglingInternalReferences(t *testing.T) {
	for _, typ := range toolTypes {
		for _, tag := range collectJSONSchemaTagDescriptions(typ) {
			lower := strings.ToLower(tag)
			if strings.Contains(lower, "tz.md") {
				t.Errorf("%s: jsonschema tag contains %q: %q", typ, "TZ.md", tag)
			}
			if sectionRefPattern.MatchString(tag) {
				t.Errorf("%s: jsonschema tag contains a %q reference: %q", typ, "section N", tag)
			}
			if strings.Contains(tag, "internal/") {
				t.Errorf("%s: jsonschema tag contains %q: %q", typ, "internal/", tag)
			}
		}
	}
}
