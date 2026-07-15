package app

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

func TestSaveLabels_validLabelingRoundTrips(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	result, err := SaveLabels(SaveLabelsRequest{
		CommentListPath: commentListPath,
		Labels: []classify.NoteLabel{
			{NoteID: 1, Label: "bug"},
			{NoteID: 2, Label: "praise"},
		},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: time.Date(2026, time.July, 15, 16, 40, 0, 0, time.UTC),
		Dir:          dir,
		Format:       artifact.FormatJSON,
	})
	if err != nil {
		t.Fatalf("SaveLabels() error = %v", err)
	}
	if len(result.Doc.Items) != 2 {
		t.Fatalf("Items = %d, want 2", len(result.Doc.Items))
	}
	for _, it := range result.Doc.Items {
		if it.Label == "" {
			t.Errorf("item %d has no label", it.ID)
		}
	}

	got, err := artifact.ReadLabeledComments(result.Path)
	if err != nil {
		t.Fatalf("ReadLabeledComments() error = %v", err)
	}
	if got.Classifier.Tool != "opencode" || got.Classifier.Model != "glm-5.2" || got.Classifier.TaxonomyVersion != classify.DefaultTaxonomyVersion {
		t.Errorf("Classifier = %+v, unexpected", got.Classifier)
	}
}

func TestSaveLabels_outOfTaxonomyLabelWritesNoFileAndReturnsValidationError(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	_, err := SaveLabels(SaveLabelsRequest{
		CommentListPath: commentListPath,
		Labels: []classify.NoteLabel{
			{NoteID: 1, Label: "not-a-real-label"},
			{NoteID: 2, Label: "praise"},
		},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: time.Now(),
		Dir:          dir,
		Format:       artifact.FormatJSON,
	})
	if err == nil {
		t.Fatalf("SaveLabels() error = nil, want a validation error")
	}
	var verr *classify.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("SaveLabels() error = %v, want *classify.ValidationError", err)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("ReadDir() error = %v", readErr)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "labeled_comments_") {
			t.Errorf("SaveLabels() left behind file %q despite a validation error", e.Name())
		}
	}
}
