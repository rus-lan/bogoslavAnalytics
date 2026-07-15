package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
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

// sampleLabeledCommentsMultiProject writes a labeled_comments artifact
// whose two bug-labeled items belong to different projects (1 and 2), so
// group-filter tests can tell "kept everything" apart from "kept only
// the requested group's project".
func sampleLabeledCommentsMultiProject(t *testing.T, dir string) string {
	t.Helper()
	at := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	doc := artifact.LabeledComments{
		Header:     artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: at}},
		Query:      artifact.CommentQuery{UserID: 42, From: domain.NewDate(2026, time.January, 1), To: domain.NewDate(2026, time.June, 30)},
		Taxonomy:   artifact.Taxonomy{Version: 1, Labels: []string{"bug", "praise", "other"}},
		Classifier: domain.Classifier{Tool: "opencode", Model: "glm-5.2", TaxonomyVersion: 1, ClassifiedAt: at},
		Items: []artifact.LabeledCommentItem{
			{MRIID: 7, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 1, ProjectID: 1, CreatedAt: at}, Label: "bug"}},
			{MRIID: 8, LabeledNote: domain.LabeledNote{Note: domain.Note{ID: 2, ProjectID: 2, CreatedAt: at}, Label: "bug"}},
		},
	}
	path := filepath.Join(dir, "labeled_comments_multiproject_test.yaml")
	if err := artifact.WriteLabeledComments(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteLabeledComments() error = %v", err)
	}
	return path
}

// TestFilterComments_groupResolvingToZeroProjectsExcludesAllItems is the
// regression guard for the empty-group bug: a group named on the request
// (req.Group != "") that resolves to zero projects (req.ProjectIDs ==
// nil/[]) must keep nothing. The old len(req.ProjectIDs) > 0 guard skips
// ByGroup entirely in this case and keeps every item instead -- this test
// fails against that guard.
func TestFilterComments_groupResolvingToZeroProjectsExcludesAllItems(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledCommentsMultiProject(t, dir)

	result, err := FilterComments(FilterCommentsRequest{
		LabeledCommentsPath: path,
		Labels:              []string{"bug"},
		Group:               "empty-group",
		ProjectIDs:          nil,
		Dir:                 dir,
	})
	if err != nil {
		t.Fatalf("FilterComments() error = %v", err)
	}
	if len(result.Doc.Items) != 0 {
		t.Errorf("Items = %d, want 0 (group %q resolved to zero projects)", len(result.Doc.Items), "empty-group")
	}
}

// TestFilterComments_noGroupRequestedKeepsAllItems confirms that leaving
// --group/group unset (req.Group == "") applies no group filtering at
// all, regardless of items spanning multiple projects.
func TestFilterComments_noGroupRequestedKeepsAllItems(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledCommentsMultiProject(t, dir)

	result, err := FilterComments(FilterCommentsRequest{
		LabeledCommentsPath: path,
		Labels:              []string{"bug"},
		Dir:                 dir,
	})
	if err != nil {
		t.Fatalf("FilterComments() error = %v", err)
	}
	if len(result.Doc.Items) != 2 {
		t.Errorf("Items = %d, want 2 (no group requested, nothing narrowed)", len(result.Doc.Items))
	}
}

// TestFilterComments_groupResolvingToProjectsFiltersCorrectly is the
// non-regression check: a group that does resolve to a non-empty project
// set still narrows down to exactly those projects.
func TestFilterComments_groupResolvingToProjectsFiltersCorrectly(t *testing.T) {
	dir := t.TempDir()
	path := sampleLabeledCommentsMultiProject(t, dir)

	result, err := FilterComments(FilterCommentsRequest{
		LabeledCommentsPath: path,
		Labels:              []string{"bug"},
		Group:               "my-group",
		ProjectIDs:          []int64{1},
		Dir:                 dir,
	})
	if err != nil {
		t.Fatalf("FilterComments() error = %v", err)
	}
	if len(result.Doc.Items) != 1 || result.Doc.Items[0].ProjectID != 1 {
		t.Fatalf("Items = %+v, want exactly one item from project 1", result.Doc.Items)
	}
}
