package cache

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func buildNotes() []domain.Note {
	return []domain.Note{
		{
			ID:           456,
			Type:         domain.NoteTypeNone,
			Body:         "looks good to me",
			Author:       domain.Author{ID: 42, Username: "alice"},
			CreatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
			NoteableID:   77,
			NoteableType: "MergeRequest",
			ProjectID:    123,
		},
		{
			ID:           457,
			Type:         domain.NoteTypeDiscussion,
			Body:         "please rename this",
			Author:       domain.Author{ID: 42, Username: "alice"},
			CreatedAt:    time.Date(2026, time.March, 1, 10, 5, 0, 0, time.UTC),
			NoteableID:   77,
			NoteableType: "MergeRequest",
			ProjectID:    123,
		},
	}
}

func TestNewLabelKey_sameItemsSameModelSameTaxonomy_hits(t *testing.T) {
	a, err := NewLabelKey(buildNotes(), "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	b, err := NewLabelKey(buildNotes(), "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	if a != b {
		t.Errorf("NewLabelKey() = %+v and %+v, want equal for identical batch/model/taxonomy", a, b)
	}
}

func TestNewLabelKey_changingModelMisses(t *testing.T) {
	a, err := NewLabelKey(buildNotes(), "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	b, err := NewLabelKey(buildNotes(), "opus-4.8", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	if a == b {
		t.Errorf("NewLabelKey() = %+v, want different key when model changes", b)
	}
}

func TestNewLabelKey_changingTaxonomyVersionMisses(t *testing.T) {
	a, err := NewLabelKey(buildNotes(), "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	b, err := NewLabelKey(buildNotes(), "glm-5.2", 4)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	if a == b {
		t.Errorf("NewLabelKey() = %+v, want different key when taxonomy_version changes", b)
	}
}

func TestNewLabelKey_changingItemContentMisses(t *testing.T) {
	a, err := NewLabelKey(buildNotes(), "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}

	changed := buildNotes()
	changed[1].Body = "please rename this variable"

	b, err := NewLabelKey(changed, "glm-5.2", 3)
	if err != nil {
		t.Fatalf("NewLabelKey() error = %v", err)
	}
	if a == b {
		t.Errorf("NewLabelKey() = %+v, want different key when item content changes", b)
	}
}
