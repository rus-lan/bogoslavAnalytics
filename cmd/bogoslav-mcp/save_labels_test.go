package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/classify"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/mcptool"
)

// writeFixtureCommentList writes a minimal, valid comment_list artifact
// under dir and returns its path, for save_labels tests that need one
// note to label.
func writeFixtureCommentList(t *testing.T, dir string) string {
	t.Helper()

	from, err := domain.ParseDate("2026-01-01")
	if err != nil {
		t.Fatalf("domain.ParseDate() error = %v", err)
	}
	to, err := domain.ParseDate("2026-06-30")
	if err != nil {
		t.Fatalf("domain.ParseDate() error = %v", err)
	}

	doc := artifact.CommentList{
		Header: artifact.Header{
			SchemaVersion: artifact.CurrentSchemaVersion,
			Kind:          artifact.KindCommentList,
			Source:        artifact.Source{GitlabURL: "https://gitlab.example.com", FetchedAt: time.Now()},
		},
		Query: artifact.CommentQuery{UserID: 1, From: from, To: to},
		Items: []artifact.CommentItem{
			{MRIID: 77, Note: domain.Note{ID: 1, Body: "looks fine", Author: domain.Author{ID: 1}, CreatedAt: time.Now(), ProjectID: 123}},
		},
	}

	path := filepath.Join(dir, "comment_list_fixture.yaml")
	if err := artifact.WriteCommentList(doc, artifact.FormatYAML, path); err != nil {
		t.Fatalf("artifact.WriteCommentList() error = %v", err)
	}
	return path
}

// TestSaveLabels_outOfTaxonomyLabelWritesNoFileAndReturnsError is the
// acceptance check for TZ.md sections 8.1 and 8.5: a labeling result
// naming a label outside the taxonomy must not produce a
// labeled_comments artifact, in any format, and must return a clear
// error.
func TestSaveLabels_outOfTaxonomyLabelWritesNoFileAndReturnsError(t *testing.T) {
	dir := t.TempDir()
	commentListPath := writeFixtureCommentList(t, dir)

	s := &toolServer{}
	_, out, err := s.saveLabels(context.Background(), nil, mcptool.SaveLabelsInput{
		FromArtifact: commentListPath,
		Labels:       []classify.NoteLabel{{NoteID: 1, Label: "not-a-real-label"}},
		Tool:         "test-tool",
		Model:        "test-model",
		ArtifactsDir: dir,
	})

	if err == nil {
		t.Fatal("saveLabels() error = nil, want an error for an out-of-taxonomy label")
	}
	if out.Path != "" {
		t.Errorf("Path = %q, want empty on a rejected labeling", out.Path)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("os.ReadDir(%q) error = %v", dir, readErr)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(commentListPath) {
			t.Errorf("unexpected file written on a rejected labeling: %s", e.Name())
		}
	}
}

// TestSaveLabels_validLabelingWritesArtifact confirms the success path
// still writes a labeled_comments artifact, so the rejection test above
// is meaningful (it is not merely that saveLabels never writes
// anything).
func TestSaveLabels_validLabelingWritesArtifact(t *testing.T) {
	dir := t.TempDir()
	commentListPath := writeFixtureCommentList(t, dir)

	s := &toolServer{}
	_, out, err := s.saveLabels(context.Background(), nil, mcptool.SaveLabelsInput{
		FromArtifact: commentListPath,
		Labels:       []classify.NoteLabel{{NoteID: 1, Label: "bug"}},
		Tool:         "test-tool",
		Model:        "test-model",
		ArtifactsDir: dir,
	})
	if err != nil {
		t.Fatalf("saveLabels() error = %v", err)
	}
	if out.Path == "" {
		t.Fatal("Path = \"\", want a written labeled_comments artifact path")
	}
	if _, statErr := os.Stat(out.Path); statErr != nil {
		t.Errorf("os.Stat(%q) error = %v, want the artifact to exist", out.Path, statErr)
	}
	if out.Count != 1 {
		t.Errorf("Count = %d, want 1", out.Count)
	}
}
