package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// sampleCommentList writes a small comment_list artifact under dir and
// returns its path, for GetClassifyBatch/SaveLabels tests.
func sampleCommentList(t *testing.T, dir string) string {
	t.Helper()
	doc := artifact.CommentList{
		Header: artifact.Header{Source: artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)}},
		Query:  artifact.CommentQuery{UserID: 42, From: domain.NewDate(2026, time.January, 1), To: domain.NewDate(2026, time.June, 30)},
		Items: []artifact.CommentItem{
			{MRIID: 7, Note: domain.Note{ID: 1, ProjectID: 1, Body: "fix the bug", Author: domain.Author{ID: 42, Username: "alice"}, CreatedAt: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)}},
			{MRIID: 7, Note: domain.Note{ID: 2, ProjectID: 1, Body: "nice catch", Author: domain.Author{ID: 42, Username: "alice"}, CreatedAt: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)}},
		},
	}
	path := filepath.Join(dir, "comment_list_test.yaml")
	if err := artifact.WriteCommentList(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}
	return path
}

func TestGetClassifyBatch_returnsTaxonomyAndDraft2020_12Schema(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	result, err := GetClassifyBatch(GetClassifyBatchRequest{CommentListPath: commentListPath, Model: "glm-5.2", Dir: dir})
	if err != nil {
		t.Fatalf("GetClassifyBatch() error = %v", err)
	}
	if result.Cached {
		t.Fatalf("GetClassifyBatch() Cached = true on first call, want false")
	}
	if len(result.Batch) != 2 {
		t.Fatalf("Batch = %d notes, want 2", len(result.Batch))
	}

	want := classify.DefaultTaxonomy()
	if result.Taxonomy.Version != want.Version || len(result.Taxonomy.Labels) != len(want.Labels) {
		t.Errorf("Taxonomy = %+v, want %+v", result.Taxonomy, want)
	}
	if result.Schema == nil {
		t.Fatalf("Schema = nil, want set")
	}
	b, err := result.Schema.MarshalJSON()
	if err != nil {
		t.Fatalf("Schema.MarshalJSON() error = %v", err)
	}
	if len(b) == 0 {
		t.Errorf("Schema.MarshalJSON() = empty")
	}
}

func TestGetClassifyBatch_promptContainsTaxonomyAndBatch(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	result, err := GetClassifyBatch(GetClassifyBatchRequest{CommentListPath: commentListPath, Model: "glm-5.2", Dir: dir})
	if err != nil {
		t.Fatalf("GetClassifyBatch() error = %v", err)
	}
	if result.Prompt == "" {
		t.Fatalf("Prompt = empty, want non-empty")
	}
	for _, label := range result.Taxonomy.Labels {
		if !strings.Contains(result.Prompt, label) {
			t.Errorf("Prompt does not contain taxonomy label %q:\n%s", label, result.Prompt)
		}
	}
	if len(result.Batch) == 0 {
		t.Fatalf("Batch = empty, want at least one note")
	}
	for _, n := range result.Batch {
		if !strings.Contains(result.Prompt, n.Body) {
			t.Errorf("Prompt does not contain batch comment body %q:\n%s", n.Body, result.Prompt)
		}
	}
}

func TestGetClassifyBatch_cachedHitOmitsPrompt(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	if _, err := SaveLabels(SaveLabelsRequest{
		CommentListPath: commentListPath,
		Labels: []classify.NoteLabel{
			{NoteID: 1, Label: "bug"},
			{NoteID: 2, Label: "praise"},
		},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: time.Now(),
		Dir:          dir,
		Format:       artifact.FormatYAML,
	}); err != nil {
		t.Fatalf("SaveLabels() error = %v", err)
	}

	result, err := GetClassifyBatch(GetClassifyBatchRequest{CommentListPath: commentListPath, Model: "glm-5.2", Dir: dir})
	if err != nil {
		t.Fatalf("GetClassifyBatch() error = %v", err)
	}
	if !result.Cached {
		t.Fatalf("Cached = false, want true")
	}
	if result.Prompt != "" {
		t.Errorf("Prompt = %q on a cache hit, want empty (no batch is handed out)", result.Prompt)
	}
}

func TestGetClassifyBatch_unchangedBatchSameModelAndTaxonomyIsCacheHit(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	saved, err := SaveLabels(SaveLabelsRequest{
		CommentListPath: commentListPath,
		Labels: []classify.NoteLabel{
			{NoteID: 1, Label: "bug"},
			{NoteID: 2, Label: "praise"},
		},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: time.Date(2026, time.July, 15, 16, 40, 0, 0, time.UTC),
		Dir:          dir,
		Format:       artifact.FormatYAML,
	})
	if err != nil {
		t.Fatalf("SaveLabels() error = %v", err)
	}

	result, err := GetClassifyBatch(GetClassifyBatchRequest{CommentListPath: commentListPath, Model: "glm-5.2", Dir: dir})
	if err != nil {
		t.Fatalf("GetClassifyBatch() error = %v", err)
	}
	if !result.Cached {
		t.Fatalf("GetClassifyBatch() Cached = false, want true")
	}
	if result.ArtifactPath != saved.Path {
		t.Errorf("ArtifactPath = %q, want %q", result.ArtifactPath, saved.Path)
	}
}

func TestGetClassifyBatch_differentModelIsNotACacheHit(t *testing.T) {
	dir := t.TempDir()
	commentListPath := sampleCommentList(t, dir)

	_, err := SaveLabels(SaveLabelsRequest{
		CommentListPath: commentListPath,
		Labels: []classify.NoteLabel{
			{NoteID: 1, Label: "bug"},
			{NoteID: 2, Label: "praise"},
		},
		Tool:         "opencode",
		Model:        "glm-5.2",
		ClassifiedAt: time.Now(),
		Dir:          dir,
		Format:       artifact.FormatYAML,
	})
	if err != nil {
		t.Fatalf("SaveLabels() error = %v", err)
	}

	result, err := GetClassifyBatch(GetClassifyBatchRequest{CommentListPath: commentListPath, Model: "a-different-model", Dir: dir})
	if err != nil {
		t.Fatalf("GetClassifyBatch() error = %v", err)
	}
	if result.Cached {
		t.Errorf("GetClassifyBatch() Cached = true for a different model, want false")
	}
}
