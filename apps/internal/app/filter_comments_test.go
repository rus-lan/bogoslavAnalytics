package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func sampleLabeledComments(t *testing.T, dir string) string {
	t.Helper()
	at := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	doc := artifact.LabeledComments{
		Header:     artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: at}},
		Query:      artifact.CommentQuery{UserID: 42, From: domain.NewDate(2026, time.January, 1), To: domain.NewDate(2026, time.June, 30)},
		Taxonomy:   artifact.Taxonomy{Version: 1, Labels: []string{"bug", "praise", "other"}},
		Classifier: domain.Classifier{Tool: "opencode", Model: "glm-5.2", TaxonomyVersion: 1, ClassifiedAt: at},
		Items: []artifact.LabeledCommentItem{
			{MRIID: 7, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}, Label: "bug"}},
			{MRIID: 7, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 2, ProjectID: 1, CreatedAt: at}, Label: "praise"}},
		},
	}
	path := filepath.Join(dir, "labeled_comments_test.yaml")
	if err := artifact.WriteLabeledComments(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}
	return path
}

func TestFilterComments_byLabelRoundTrips(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledComments(t, dir)

	result, err := FilterComments(FilterCommentsRequest{
		LabeledCommentsPath: path,
		Labels:              []string{"bug"},
		Dir:                 dir,
		Format:              artifact.FormatJSON,
	})
	if err != nil {
		t.Fatalf("FilterComments() error = %v", err)
	}
	if len(result.Doc.Items) != 1 || result.Doc.Items[0].Label != "bug" {
		t.Fatalf("Items = %+v, want exactly the bug-labeled item", result.Doc.Items)
	}

	got, err := artifact.ReadFilteredComments(result.Path)
	if err != nil {
		t.Fatalf("ReadFilteredComments() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Errorf("ReadFilteredComments() items = %d, want 1", len(got.Items))
	}
}

func TestFilterComments_projectFilterNarrowsFurther(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledComments(t, dir)

	otherProject := int64(999)
	result, err := FilterComments(FilterCommentsRequest{
		LabeledCommentsPath: path,
		Labels:              []string{"bug", "praise"},
		ProjectID:           &otherProject,
		Dir:                 dir,
	})
	if err != nil {
		t.Fatalf("FilterComments() error = %v", err)
	}
	if len(result.Doc.Items) != 0 {
		t.Errorf("Items = %d, want 0 (no rows belong to project %d)", len(result.Doc.Items), otherProject)
	}
}

func TestFilterComments_noLabelsIsError(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledComments(t, dir)

	_, err := FilterComments(FilterCommentsRequest{LabeledCommentsPath: path, Dir: dir})
	if !errors.Is(err, ErrNoLabels) {
		t.Errorf("error = %v, want ErrNoLabels", err)
	}
}
