package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func sampleClassifier() domain.Classifier {
	return domain.Classifier{
		Tool:            "opencode",
		Model:           "glm-5.2",
		TaxonomyVersion: 3,
		ClassifiedAt:    time.Date(2026, time.July, 15, 16, 40, 0, 0, time.UTC),
	}
}

func sampleLabeledComments() LabeledComments {
	comments := sampleCommentList()
	return LabeledComments{
		Header: comments.Header,
		Query:  comments.Query,
		Taxonomy: Taxonomy{
			Version: 3,
			Labels: []string{
				"bug", "style", "naming", "architecture", "performance",
				"security", "test", "docs", "question", "nitpick", "praise", "other",
			},
		},
		Classifier: sampleClassifier(),
		Items: []LabeledCommentItem{
			{
				MRIID: comments.Items[0].MRIID,
				LabeledNote: domain.LabeledNote{
					Note:  comments.Items[0].Note,
					Label: "style",
				},
			},
			{
				MRIID: comments.Items[1].MRIID,
				LabeledNote: domain.LabeledNote{
					Note:  comments.Items[1].Note,
					Label: "other",
				},
			},
		},
	}
}

func TestWriteLabeledComments_jsonRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  LabeledComments
	}{
		{"full taxonomy and classifier", sampleLabeledComments()},
		{"no items", func() LabeledComments {
			d := sampleLabeledComments()
			d.Items = nil
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "labeled_comments_test.json")
			if err := WriteLabeledComments(tc.doc, FormatJSON, path); err != nil {
				t.Fatalf("WriteLabeledComments() error = %v", err)
			}

			got, err := ReadLabeledComments(path)
			if err != nil {
				t.Fatalf("ReadLabeledComments() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindLabeledComments
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteLabeledComments_yamlRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  LabeledComments
	}{
		{"full taxonomy and classifier", sampleLabeledComments()},
		{"no items", func() LabeledComments {
			d := sampleLabeledComments()
			d.Items = nil
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "labeled_comments_test.yaml")
			if err := WriteLabeledComments(tc.doc, FormatYAML, path); err != nil {
				t.Fatalf("WriteLabeledComments() error = %v", err)
			}

			got, err := ReadLabeledComments(path)
			if err != nil {
				t.Fatalf("ReadLabeledComments() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindLabeledComments
			assertEqualJSON(t, got, want)
		})
	}
}

// TestWriteLabeledComments_provenanceRoundTrip is the direct check for
// TZ.md section 8.3: the classifier block's four fields (tool, model,
// taxonomy_version, classified_at) must survive a round trip in both
// formats.
func TestWriteLabeledComments_provenanceRoundTrip(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			ext, err := format.Extension()
			if err != nil {
				t.Fatalf("Extension() error = %v", err)
			}
			path := filepath.Join(t.TempDir(), "labeled_comments_provenance."+ext)

			doc := sampleLabeledComments()
			if err := WriteLabeledComments(doc, format, path); err != nil {
				t.Fatalf("WriteLabeledComments() error = %v", err)
			}

			got, err := ReadLabeledComments(path)
			if err != nil {
				t.Fatalf("ReadLabeledComments() error = %v", err)
			}

			want := sampleClassifier()
			if got.Classifier.Tool != want.Tool {
				t.Errorf("Classifier.Tool = %q, want %q", got.Classifier.Tool, want.Tool)
			}
			if got.Classifier.Model != want.Model {
				t.Errorf("Classifier.Model = %q, want %q", got.Classifier.Model, want.Model)
			}
			if got.Classifier.TaxonomyVersion != want.TaxonomyVersion {
				t.Errorf("Classifier.TaxonomyVersion = %d, want %d", got.Classifier.TaxonomyVersion, want.TaxonomyVersion)
			}
			if !got.Classifier.ClassifiedAt.Equal(want.ClassifiedAt) {
				t.Errorf("Classifier.ClassifiedAt = %v, want %v", got.Classifier.ClassifiedAt, want.ClassifiedAt)
			}
		})
	}
}

func TestWriteLabeledComments_requiresClassifier(t *testing.T) {
	cases := []struct {
		name       string
		classifier domain.Classifier
	}{
		{"zero value classifier", domain.Classifier{}},
		{"missing tool", func() domain.Classifier {
			c := sampleClassifier()
			c.Tool = ""
			return c
		}()},
		{"missing model", func() domain.Classifier {
			c := sampleClassifier()
			c.Model = ""
			return c
		}()},
		{"zero taxonomy version", func() domain.Classifier {
			c := sampleClassifier()
			c.TaxonomyVersion = 0
			return c
		}()},
		{"zero classified_at", func() domain.Classifier {
			c := sampleClassifier()
			c.ClassifiedAt = time.Time{}
			return c
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "labeled_comments_test.yaml")

			doc := sampleLabeledComments()
			doc.Classifier = tc.classifier

			err := WriteLabeledComments(doc, FormatYAML, path)
			if !errors.Is(err, ErrMissingClassifier) {
				t.Fatalf("WriteLabeledComments() error = %v, want ErrMissingClassifier", err)
			}

			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("WriteLabeledComments() left a file behind at %q, want no file on validation failure", path)
			}
		})
	}
}

func TestWriteLabeledComments_textFormatProducesReadableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labeled_comments_test.txt")
	if err := WriteLabeledComments(sampleLabeledComments(), FormatText, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	text := string(data)
	if len(text) == 0 {
		t.Fatal("text output is empty")
	}
	if !strings.Contains(text, "labeled_comments") {
		t.Errorf("text output = %q, want it to mention kind labeled_comments", text)
	}
	if !strings.Contains(text, "glm-5.2") {
		t.Errorf("text output = %q, want classifier model rendered", text)
	}
	if !strings.Contains(text, "style") {
		t.Errorf("text output = %q, want item labels rendered", text)
	}
}

func TestReadLabeledComments_rejectsTextFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labeled_comments_test.txt")
	if err := WriteLabeledComments(sampleLabeledComments(), FormatText, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}

	_, err := ReadLabeledComments(path)
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("ReadLabeledComments() error = %v, want ErrNotReadable", err)
	}
}
