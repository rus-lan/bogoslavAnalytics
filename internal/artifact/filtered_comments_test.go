package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func sampleFilteredComments() FilteredComments {
	labeled := sampleLabeledComments()
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.June, 30)
	return FilteredComments{
		Header: labeled.Header,
		Query: FilteredQuery{
			FromArtifact: "artifacts/labeled_comments_def456.yaml",
			Labels:       []string{"style", "bug"},
			From:         &from,
			To:           &to,
			Group:        "my-group",
			Project:      "my-group/repo",
		},
		Items: []LabeledCommentItem{labeled.Items[0]},
	}
}

func TestWriteFilteredComments_jsonRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  FilteredComments
	}{
		{"full query with optional filters", sampleFilteredComments()},
		{"no optional date/group/project filters", func() FilteredComments {
			d := sampleFilteredComments()
			d.Query.From = nil
			d.Query.To = nil
			d.Query.Group = ""
			d.Query.Project = ""
			return d
		}()},
		{"no items", func() FilteredComments {
			d := sampleFilteredComments()
			d.Items = nil
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "filtered_comments_test.json")
			if err := WriteFilteredComments(tc.doc, FormatJSON, path); err != nil {
				t.Fatalf("WriteFilteredComments() error = %v", err)
			}

			got, err := ReadFilteredComments(path)
			if err != nil {
				t.Fatalf("ReadFilteredComments() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindFilteredComments
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteFilteredComments_yamlRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  FilteredComments
	}{
		{"full query with optional filters", sampleFilteredComments()},
		{"no optional date/group/project filters", func() FilteredComments {
			d := sampleFilteredComments()
			d.Query.From = nil
			d.Query.To = nil
			d.Query.Group = ""
			d.Query.Project = ""
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "filtered_comments_test.yaml")
			if err := WriteFilteredComments(tc.doc, FormatYAML, path); err != nil {
				t.Fatalf("WriteFilteredComments() error = %v", err)
			}

			got, err := ReadFilteredComments(path)
			if err != nil {
				t.Fatalf("ReadFilteredComments() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindFilteredComments
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteFilteredComments_textFormatProducesReadableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filtered_comments_test.txt")
	if err := WriteFilteredComments(sampleFilteredComments(), FormatText, path); err != nil {
		t.Fatalf("WriteFilteredComments() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	text := string(data)
	if len(text) == 0 {
		t.Fatal("text output is empty")
	}
	if !strings.Contains(text, "filtered_comments") {
		t.Errorf("text output = %q, want it to mention kind filtered_comments", text)
	}
	if !strings.Contains(text, "style") {
		t.Errorf("text output = %q, want labels rendered", text)
	}
}

func TestReadFilteredComments_rejectsTextFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filtered_comments_test.txt")
	if err := WriteFilteredComments(sampleFilteredComments(), FormatText, path); err != nil {
		t.Fatalf("WriteFilteredComments() error = %v", err)
	}

	_, err := ReadFilteredComments(path)
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("ReadFilteredComments() error = %v, want ErrNotReadable", err)
	}
}
