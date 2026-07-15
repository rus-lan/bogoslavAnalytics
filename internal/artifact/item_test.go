package artifact

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMRItem_projectPathOmittedWhenEmpty locks in that ProjectPath is
// optional on the wire: a writer with no path to give (there is no
// producer for it yet, see the doc comment on MRItem) must not emit an
// empty "project_path" field, the same way Title, WebURL, CreatedAt and
// UpdatedAt are already omitted when unset.
func TestMRItem_projectPathOmittedWhenEmpty(t *testing.T) {
	item := MRItem{ProjectID: 5, MRIID: 9, CommentCount: 2}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if strings.Contains(string(b), "project_path") {
		t.Errorf("json = %s, want no project_path key when ProjectPath is empty", b)
	}
	for _, want := range []string{`"project_id":5`, `"mr_iid":9`, `"comment_count":2`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("json = %s, want it to contain %s", b, want)
		}
	}
}

// TestMRItem_projectPathPresentWhenSet is the mirror check: an MRItem
// that does carry a path still writes it, so existing fixtures and
// future producers are unaffected by making the field optional.
func TestMRItem_projectPathPresentWhenSet(t *testing.T) {
	item := MRItem{ProjectID: 5, ProjectPath: "my-group/repo", MRIID: 9, CommentCount: 2}

	b, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	if !strings.Contains(string(b), `"project_path":"my-group/repo"`) {
		t.Errorf("json = %s, want project_path to be present", b)
	}
}

// TestWriteMRList_projectPathOmittedFromWireWhenEmpty checks the field
// through the real write path, in both formats that round-trip
// (json, yaml): an item with an empty ProjectPath must not put an
// empty project_path on the wire, and must read back as the zero
// value.
func TestWriteMRList_projectPathOmittedFromWireWhenEmpty(t *testing.T) {
	doc := sampleMRList()
	doc.Items = []MRItem{{ProjectID: 123, MRIID: 77, CommentCount: 8}}

	for _, format := range []Format{FormatJSON, FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mr_list_test."+string(format))
			if err := WriteMRList(doc, format, path); err != nil {
				t.Fatalf("WriteMRList() error = %v", err)
			}

			rawBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read written file: %v", err)
			}
			if strings.Contains(string(rawBytes), "project_path") {
				t.Errorf("%s output contains project_path with an empty ProjectPath:\n%s", format, rawBytes)
			}

			got, err := ReadMRList(path)
			if err != nil {
				t.Fatalf("ReadMRList() error = %v", err)
			}
			if len(got.Items) != 1 || got.Items[0].ProjectPath != "" {
				t.Errorf("ReadMRList().Items = %+v, want one item with empty ProjectPath", got.Items)
			}
			if len(got.Items) == 1 && (got.Items[0].ProjectID != 123 || got.Items[0].MRIID != 77 || got.Items[0].CommentCount != 8) {
				t.Errorf("ReadMRList().Items[0] = %+v, want ProjectID=123 MRIID=77 CommentCount=8", got.Items[0])
			}
		})
	}
}

// TestWriteMRList_projectPathRoundTripsWhenSet checks that fixtures
// which do carry a ProjectPath are unaffected by the field becoming
// optional: it still round-trips through both formats.
func TestWriteMRList_projectPathRoundTripsWhenSet(t *testing.T) {
	doc := sampleMRList()

	for _, format := range []Format{FormatJSON, FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mr_list_test."+string(format))
			if err := WriteMRList(doc, format, path); err != nil {
				t.Fatalf("WriteMRList() error = %v", err)
			}

			rawBytes, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read written file: %v", err)
			}
			if !strings.Contains(string(rawBytes), "project_path") {
				t.Errorf("%s output missing project_path for an item that set it:\n%s", format, rawBytes)
			}

			got, err := ReadMRList(path)
			if err != nil {
				t.Fatalf("ReadMRList() error = %v", err)
			}
			for i, item := range got.Items {
				if item.ProjectPath != doc.Items[i].ProjectPath {
					t.Errorf("Items[%d].ProjectPath = %q, want %q", i, item.ProjectPath, doc.Items[i].ProjectPath)
				}
			}
		})
	}
}
