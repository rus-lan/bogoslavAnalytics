package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

func TestNewSaveLabelsRequest_mapsFlagsToRequest(t *testing.T) {
	cmd, flags := buildFlags(t, registerSaveLabelsFlags, []string{
		"--from-artifact", "artifacts/comment_list_x.yaml",
		"--labels", "labels.json",
		"--tool", "opencode",
		"--model", "glm-5.2",
		"--format", "json",
	})
	_ = cmd

	labels := []classify.NoteLabel{{NoteID: 1, Label: "bug"}}
	classifiedAt := time.Date(2026, time.July, 15, 16, 40, 0, 0, time.UTC)

	got, err := newSaveLabelsRequest(*flags, labels, nil, classifiedAt)
	if err != nil {
		t.Fatalf("newSaveLabelsRequest() error = %v", err)
	}
	want := app.SaveLabelsRequest{
		CommentListPath: "artifacts/comment_list_x.yaml",
		Labels:          labels,
		Tool:            "opencode",
		Model:           "glm-5.2",
		ClassifiedAt:    classifiedAt,
		Format:          artifact.FormatJSON,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newSaveLabelsRequest() = %+v, want %+v", got, want)
	}
}

func TestReadNoteLabels_fromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "labels.json")
	writeFileT(t, path, `[{"note_id": 1, "label": "bug"}, {"note_id": 2, "label": "style"}]`)

	cmd, _ := buildFlags(t, registerSaveLabelsFlags, nil)
	got, err := readNoteLabels(cmd, path)
	if err != nil {
		t.Fatalf("readNoteLabels() error = %v", err)
	}
	want := []classify.NoteLabel{{NoteID: 1, Label: "bug"}, {NoteID: 2, Label: "style"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readNoteLabels() = %+v, want %+v", got, want)
	}
}

func TestReadNoteLabels_fromStdin(t *testing.T) {
	cmd, _ := buildFlags(t, registerSaveLabelsFlags, nil)
	cmd.SetIn(bytes.NewBufferString(`[{"note_id": 1, "label": "bug"}]`))

	got, err := readNoteLabels(cmd, "-")
	if err != nil {
		t.Fatalf("readNoteLabels() error = %v", err)
	}
	want := []classify.NoteLabel{{NoteID: 1, Label: "bug"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readNoteLabels() = %+v, want %+v", got, want)
	}
}

func TestNewSaveLabelsRequest_rejectsUnknownFormat(t *testing.T) {
	_, flags := buildFlags(t, registerSaveLabelsFlags, []string{
		"--from-artifact", "x.yaml", "--labels", "l.json", "--tool", "t", "--model", "m", "--format", "xml",
	})
	_, err := newSaveLabelsRequest(*flags, nil, nil, time.Now())
	if err == nil {
		t.Fatal("newSaveLabelsRequest() error = nil, want an error for --format xml")
	}
}
