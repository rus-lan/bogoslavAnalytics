package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tamperSchemaVersion rewrites the written schema_version value of a
// valid artifact file to 99, to simulate reading a file written by a
// future, unknown schema version.
func tamperSchemaVersion(t *testing.T, path string, format Format) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	text := string(data)

	var old, replacement string
	switch format {
	case FormatJSON:
		old, replacement = `"schema_version": 1`, `"schema_version": 99`
	case FormatYAML:
		old, replacement = "schema_version: 1\n", "schema_version: 99\n"
	default:
		t.Fatalf("tamperSchemaVersion: unsupported format %q", format)
	}

	if !strings.Contains(text, old) {
		t.Fatalf("tamperSchemaVersion: substring %q not found in:\n%s", old, text)
	}
	text = strings.Replace(text, old, replacement, 1)

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

// TestRead_rejectsUnknownSchemaVersion checks, for all four artifact
// kinds and both readable formats, that a schema_version other than
// CurrentSchemaVersion is rejected with ErrUnknownSchemaVersion
// (TZ.md section 4).
func TestRead_rejectsUnknownSchemaVersion(t *testing.T) {
	for _, k := range allKindCases() {
		for _, format := range []Format{FormatJSON, FormatYAML} {
			t.Run(k.name+"/"+string(format), func(t *testing.T) {
				ext, err := format.Extension()
				if err != nil {
					t.Fatalf("Extension() error = %v", err)
				}
				path := filepath.Join(t.TempDir(), k.name+"_test."+ext)

				if err := k.write(format, path); err != nil {
					t.Fatalf("write() error = %v", err)
				}
				tamperSchemaVersion(t, path, format)

				err = k.read(path)
				if !errors.Is(err, ErrUnknownSchemaVersion) {
					t.Errorf("read() error = %v, want ErrUnknownSchemaVersion", err)
				}
			})
		}
	}
}

// TestReadMRList_rejectsWrongKind checks that reading a comment_list
// file with ReadMRList (kind mismatch) fails clearly instead of
// silently returning a half-populated MRList.
func TestReadMRList_rejectsWrongKind(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comment_list_test.yaml")
	if err := WriteCommentList(sampleCommentList(), FormatYAML, path); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	_, err := ReadMRList(path)
	if !errors.Is(err, ErrKindMismatch) {
		t.Errorf("ReadMRList() error = %v, want ErrKindMismatch", err)
	}
}

// TestReadMRList_chainsIntoCommentQuery exercises the from_artifact
// chaining mechanic (TZ.md section 4.2): a comment_list step reads a
// prior mr_list artifact and records the source path plus the merge
// requests it carried forward.
func TestReadMRList_chainsIntoCommentQuery(t *testing.T) {
	dir := t.TempDir()
	mrListPath := filepath.Join(dir, "mr_list_abc123.yaml")
	if err := WriteMRList(sampleMRList(), FormatYAML, mrListPath); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	mrList, err := ReadMRList(mrListPath)
	if err != nil {
		t.Fatalf("ReadMRList() error = %v", err)
	}

	mrs := make([]MRRef, len(mrList.Items))
	for i, item := range mrList.Items {
		mrs[i] = MRRef{ProjectID: item.ProjectID, MRIID: item.MRIID}
	}

	commentDoc := sampleCommentList()
	commentDoc.Query.MRs = mrs
	commentDoc.Query.FromArtifact = mrListPath

	commentListPath := filepath.Join(dir, "comment_list_def456.yaml")
	if err := WriteCommentList(commentDoc, FormatYAML, commentListPath); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	got, err := ReadCommentList(commentListPath)
	if err != nil {
		t.Fatalf("ReadCommentList() error = %v", err)
	}
	if got.Query.FromArtifact != mrListPath {
		t.Errorf("Query.FromArtifact = %q, want %q", got.Query.FromArtifact, mrListPath)
	}
	if len(got.Query.MRs) != len(mrList.Items) {
		t.Fatalf("len(Query.MRs) = %d, want %d", len(got.Query.MRs), len(mrList.Items))
	}
	for i, ref := range got.Query.MRs {
		want := MRRef{ProjectID: mrList.Items[i].ProjectID, MRIID: mrList.Items[i].MRIID}
		if ref != want {
			t.Errorf("Query.MRs[%d] = %+v, want %+v", i, ref, want)
		}
	}
}

func TestFormatFromPath_infersFormatFromExtension(t *testing.T) {
	cases := []struct {
		path string
		want Format
	}{
		{"artifacts/mr_list_abc.json", FormatJSON},
		{"artifacts/mr_list_abc.yaml", FormatYAML},
		{"artifacts/mr_list_abc.yml", FormatYAML},
		{"artifacts/mr_list_abc.txt", FormatText},
		{"artifacts/mr_list_abc.html", FormatHTML},
		{"artifacts/mr_list_abc.htm", FormatHTML},
		{"artifacts/mr_list_abc.JSON", FormatJSON},
		{"artifacts/mr_list_abc.HTML", FormatHTML},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, err := FormatFromPath(tc.path)
			if err != nil {
				t.Fatalf("FormatFromPath() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("FormatFromPath(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestFormatFromPath_rejectsUnknownExtension(t *testing.T) {
	_, err := FormatFromPath("artifacts/mr_list_abc.csv")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("FormatFromPath() error = %v, want ErrUnsupportedFormat", err)
	}
}

func TestFormat_Extension_knownAndUnknownFormats(t *testing.T) {
	cases := []struct {
		format  Format
		want    string
		wantErr bool
	}{
		{FormatJSON, "json", false},
		{FormatYAML, "yaml", false},
		{FormatText, "txt", false},
		{FormatHTML, "html", false},
		{Format("csv"), "", true},
	}
	for _, tc := range cases {
		t.Run(string(tc.format), func(t *testing.T) {
			got, err := tc.format.Extension()
			if tc.wantErr {
				if !errors.Is(err, ErrUnsupportedFormat) {
					t.Fatalf("Extension() error = %v, want ErrUnsupportedFormat", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Extension() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("Extension() = %q, want %q", got, tc.want)
			}
		})
	}
}
