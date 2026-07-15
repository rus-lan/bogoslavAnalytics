package contracts

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
)

// committedOpenAPIPath is contracts/openapi.yaml's path, relative to this
// package's own directory (go test's working directory is always the
// package directory it was invoked for).
const committedOpenAPIPath = "../../../contracts/openapi.yaml"

// TestGenerate_isDeterministic is the zero-diff regeneration guarantee
// itself (TZ.md section 12.16): two independent calls to Generate, with
// nothing else changed, must produce byte-identical output.
func TestGenerate_isDeterministic(t *testing.T) {
	a, err := Generate()
	if err != nil {
		t.Fatalf("Generate() (1st call) error = %v", err)
	}
	b, err := Generate()
	if err != nil {
		t.Fatalf("Generate() (2nd call) error = %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("Generate() is not deterministic: two calls produced different bytes")
	}
}

// TestGenerate_matchesCommittedFile is the guard TZ.md section 12.16
// asks for by name: a stale checked-in contracts/openapi.yaml must fail
// this test. It reads the file committed at committedOpenAPIPath and
// compares it, byte for byte, against what Generate produces right now
// from the current Go types.
func TestGenerate_matchesCommittedFile(t *testing.T) {
	want, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	got, err := os.ReadFile(committedOpenAPIPath)
	if err != nil {
		t.Fatalf("read committed %s: %v (run `make -C apps contracts` to generate it)", committedOpenAPIPath, err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("committed %s is stale: does not match Generate()'s current output; "+
			"run `make -C apps contracts` and commit the result", committedOpenAPIPath)
	}
}

// parsedDocument is the small slice of the OpenAPI 3.1 document these
// tests need to inspect: the openapi version field, the (expected
// empty) paths object, and components/schemas as a generic map, keyed
// by schema name, each schema itself a generic map so tests can check
// individual JSON Schema keywords (type, required, and so on) without
// needing jsonschema-go's own Schema type (which would re-decode
// $defs/properties/items in a way this package's own generation no
// longer needs once the document is written).
type parsedDocument struct {
	OpenAPI    string         `yaml:"openapi"`
	Paths      map[string]any `yaml:"paths"`
	Components struct {
		Schemas map[string]map[string]any `yaml:"schemas"`
	} `yaml:"components"`
}

func mustParse(t *testing.T, data []byte) parsedDocument {
	t.Helper()
	var doc parsedDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Generate() output does not parse as YAML: %v", err)
	}
	return doc
}

// TestGenerate_isOpenAPI31WithEmptyPaths checks the structural
// invariants TZ.md section 12.16 asks for directly: the document parses
// as YAML, its openapi field is a 3.1.x version, and paths is present
// and empty (TZ.md section 10: no REST API, TZ.md section 13). This is
// a hand-rolled structural check, not a full OpenAPI validator -- see
// this task's own report for why no available Go OpenAPI validator
// library was used as a substitute.
func TestGenerate_isOpenAPI31WithEmptyPaths(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	doc := mustParse(t, data)

	if !strings.HasPrefix(doc.OpenAPI, "3.1.") {
		t.Errorf("openapi = %q, want a 3.1.x version", doc.OpenAPI)
	}
	if doc.Paths == nil {
		t.Errorf("paths is absent, want present and empty")
	}
	if len(doc.Paths) != 0 {
		t.Errorf("paths = %v, want empty (TZ.md section 10: no REST API)", doc.Paths)
	}
}

// toolInputSchemaNames are the six MCP tool input types' component
// schema names (TZ.md section 7.2).
var toolInputSchemaNames = []string{
	"FindMRsInput",
	"GetCommentsInput",
	"GetClassifyBatchInput",
	"SaveLabelsInput",
	"FilterCommentsInput",
	"GetStatsInput",
}

// artifactSchemaNames are the four artifact document types' component
// schema names (TZ.md section 4), corresponding to kinds mr_list,
// comment_list, labeled_comments and filtered_comments respectively.
var artifactSchemaNames = []string{
	"MRList",
	"CommentList",
	"LabeledComments",
	"FilteredComments",
}

// TestGenerate_containsAllSixToolInputSchemas is the acceptance check
// for TZ.md section 12.16's "every one of the six tool inputs is
// present in components/schemas" -- and each one is type object and
// draft-2020-12 shaped: no $schema keyword pinned to anything else (an
// absent $schema means the library's documented 2020-12 default
// applies, exactly as apps/internal/mcptool/schema_test.go already
// established for the same six types).
func TestGenerate_containsAllSixToolInputSchemas(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	doc := mustParse(t, data)

	for _, name := range toolInputSchemaNames {
		t.Run(name, func(t *testing.T) {
			schema, ok := doc.Components.Schemas[name]
			if !ok {
				t.Fatalf("components/schemas is missing %q", name)
			}
			if schema["type"] != "object" {
				t.Errorf("%s.type = %v, want %q", name, schema["type"], "object")
			}
			if v, has := schema["$schema"]; has {
				t.Errorf("%s.$schema = %v, want absent (unset means 2020-12 by default)", name, v)
			}
		})
	}
}

// TestGenerate_containsAllFourArtifactSchemas is the acceptance check
// for the four artifact schemas (mr_list, comment_list,
// labeled_comments, filtered_comments -- TZ.md section 4).
func TestGenerate_containsAllFourArtifactSchemas(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	doc := mustParse(t, data)

	for _, name := range artifactSchemaNames {
		t.Run(name, func(t *testing.T) {
			schema, ok := doc.Components.Schemas[name]
			if !ok {
				t.Fatalf("components/schemas is missing %q", name)
			}
			if schema["type"] != "object" {
				t.Errorf("%s.type = %v, want %q", name, schema["type"], "object")
			}
		})
	}
}

// TestGenerate_findMRsInputRequiresMoreThan is a source-grounded check,
// mirroring apps/internal/mcptool/schema_test.go's own
// TestFindMRsInput_moreThanIsRequired, that this package's generated
// document -- not just a bare jsonschema.ForType call -- keeps
// more_than required: FindMRsInput has no omitempty/omitzero tag on
// that field (TZ.md section 7.2 lists it without "?", unlike
// group/project/mr/strict).
func TestGenerate_findMRsInputRequiresMoreThan(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	doc := mustParse(t, data)

	schema, ok := doc.Components.Schemas["FindMRsInput"]
	if !ok {
		t.Fatalf("components/schemas is missing %q", "FindMRsInput")
	}
	required, _ := schema["required"].([]any)
	found := false
	for _, r := range required {
		if r == "more_than" {
			found = true
		}
	}
	if !found {
		t.Errorf("FindMRsInput.required = %v, want it to include %q", required, "more_than")
	}
}

// TestGenerate_omitsGetClassifyBatchOutput documents, as an executable
// check, the one output schema this package leaves out on purpose
// (doc.go, generate.go's checkGetClassifyBatchOutputCycle): its Go type
// embeds *jsonschema.Schema, which is self-referential, so
// jsonschema.ForType cannot infer a schema for it. This asserts the
// omission is exactly that -- nothing named GetClassifyBatchOutput ever
// silently appears (which would mean, most likely, someone swapped in
// a hand-written schema for it, which TZ.md section 10 forbids).
func TestGenerate_omitsGetClassifyBatchOutput(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	doc := mustParse(t, data)

	if _, ok := doc.Components.Schemas["GetClassifyBatchOutput"]; ok {
		t.Errorf("components/schemas unexpectedly contains GetClassifyBatchOutput; " +
			"this type's self-reference cycle (see doc.go) was believed to make it " +
			"uninferable -- if that changed, checkGetClassifyBatchOutputCycle should " +
			"already have failed loudly instead of silently starting to include this")
	}
}

// TestCheckGetClassifyBatchOutputCycle_holdsToday is a direct,
// package-internal check that the one assumption Generate leans on for
// its documented omission -- jsonschema.ForType still fails on
// GetClassifyBatchOutput specifically because of a self-reference cycle
// -- holds against the jsonschema-go version this module currently
// depends on.
func TestCheckGetClassifyBatchOutputCycle_holdsToday(t *testing.T) {
	if err := checkGetClassifyBatchOutputCycle(); err != nil {
		t.Fatalf("checkGetClassifyBatchOutputCycle() = %v, want nil (the self-reference cycle "+
			"this package's design relies on no longer holds -- see doc.go)", err)
	}
}

// TestSchemaGeneration_reflectsGoTypeShape_notHardcoded proves that the
// exact mechanism Generate uses per entry -- jsonschema.ForType followed
// by schemaNode -- is a pure function of the Go type's shape, not a
// hardcoded or cached string: changing a type from widgetV1 to
// widgetV2, which differs by exactly one added field, changes the
// resulting schema, and the new field's name shows up in it. This uses
// two package-local fixture types instead of editing a real
// mcptool/artifact type: this change's scope is limited to the new
// generator package and contracts/openapi.yaml, and TZ.md sections 2.4
// and this task both treat every existing internal/ package's
// behavior, mcptool's and artifact's included, as off limits.
func TestSchemaGeneration_reflectsGoTypeShape_notHardcoded(t *testing.T) {
	type widgetV1 struct {
		A string `json:"a"`
	}
	type widgetV2 struct {
		A string `json:"a"`
		B int    `json:"b,omitempty"`
	}

	s1, err := jsonschema.ForType(reflect.TypeFor[widgetV1](), &jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("jsonschema.ForType(widgetV1) error = %v", err)
	}
	s2, err := jsonschema.ForType(reflect.TypeFor[widgetV2](), &jsonschema.ForOptions{})
	if err != nil {
		t.Fatalf("jsonschema.ForType(widgetV2) error = %v", err)
	}

	n1, err := schemaNode(s1)
	if err != nil {
		t.Fatalf("schemaNode(widgetV1) error = %v", err)
	}
	n2, err := schemaNode(s2)
	if err != nil {
		t.Fatalf("schemaNode(widgetV2) error = %v", err)
	}

	b1, err := yaml.Marshal(n1)
	if err != nil {
		t.Fatalf("yaml.Marshal(widgetV1 node) error = %v", err)
	}
	b2, err := yaml.Marshal(n2)
	if err != nil {
		t.Fatalf("yaml.Marshal(widgetV2 node) error = %v", err)
	}

	if bytes.Equal(b1, b2) {
		t.Fatalf("schema output did not change when the Go type gained a field %q -> %q; "+
			"looks hardcoded, not generated", "widgetV1", "widgetV2")
	}
	if !bytes.Contains(b2, []byte("b:")) {
		t.Errorf("widgetV2's schema does not mention its new field %q:\n%s", "b", b2)
	}
	if bytes.Contains(b1, []byte("b:")) {
		t.Errorf("widgetV1's schema unexpectedly mentions field %q, which it does not have:\n%s", "b", b1)
	}
}

// TestGenerate_schemaKeysAreSorted checks the "sort keys" half of TZ.md
// section 12.16's determinism requirement directly against Generate's
// own output text, rather than only trusting gopkg.in/yaml.v3's map-key
// sort behavior: components/schemas' keys must already appear in
// ascending order in the rendered YAML.
func TestGenerate_schemaKeysAreSorted(t *testing.T) {
	data, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var names []string
	inSchemas := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case strings.HasPrefix(line, "components:"):
			continue
		case strings.HasPrefix(line, "    schemas:"):
			inSchemas = true
			continue
		case inSchemas && strings.HasPrefix(line, "        ") && !strings.HasPrefix(line, "         "):
			names = append(names, strings.TrimSuffix(strings.TrimSpace(line), ":"))
		case inSchemas && line != "" && !strings.HasPrefix(line, " "):
			inSchemas = false
		}
	}

	if len(names) == 0 {
		t.Fatalf("found no components/schemas entries to check order of")
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("components/schemas keys are not sorted ascending: %q appears before %q", names[i-1], names[i])
		}
	}
}
