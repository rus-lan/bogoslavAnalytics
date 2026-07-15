package artifact

import (
	"encoding/json"
	"testing"
)

// assertEqualJSON compares got and want by re-marshaling both to
// indented JSON rather than with reflect.DeepEqual, since time.Time is
// documented as unsafe to compare directly (see also
// internal/domain/note_test.go, which follows the same convention).
func assertEqualJSON[T any](t *testing.T, got, want T) {
	t.Helper()

	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("round trip mismatch:\n got = %s\nwant = %s", gotJSON, wantJSON)
	}
}

// kindCase names one of the four artifact kinds together with closures
// that write and read a sample document of that kind. It lets
// cross-cutting tests (schema version, write-only formats, ...) run
// the same check against all four kinds without duplicating the
// per-kind Write/Read calls.
type kindCase struct {
	name  string
	write func(format Format, path string) error
	read  func(path string) error
}

// allKindCases returns one kindCase per artifact kind, each backed by
// that kind's sample fixture (sampleMRList, sampleCommentList, ...).
func allKindCases() []kindCase {
	return []kindCase{
		{
			name:  "mr_list",
			write: func(format Format, path string) error { return WriteMRList(sampleMRList(), format, path) },
			read:  func(path string) error { _, err := ReadMRList(path); return err },
		},
		{
			name:  "comment_list",
			write: func(format Format, path string) error { return WriteCommentList(sampleCommentList(), format, path) },
			read:  func(path string) error { _, err := ReadCommentList(path); return err },
		},
		{
			name: "labeled_comments",
			write: func(format Format, path string) error {
				return WriteLabeledComments(sampleLabeledComments(), format, path)
			},
			read: func(path string) error { _, err := ReadLabeledComments(path); return err },
		},
		{
			name: "filtered_comments",
			write: func(format Format, path string) error {
				return WriteFilteredComments(sampleFilteredComments(), format, path)
			},
			read: func(path string) error { _, err := ReadFilteredComments(path); return err },
		},
	}
}
