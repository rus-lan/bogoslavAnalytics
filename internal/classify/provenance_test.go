package classify

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// TestNewClassifier_carriesAllFourFields is the acceptance check for
// TZ.md section 8.3: the classifier provenance block carries tool,
// model, taxonomy_version, and classified_at.
func TestNewClassifier_carriesAllFourFields(t *testing.T) {
	classifiedAt := time.Date(2026, time.July, 15, 16, 40, 0, 0, time.UTC)

	got := NewClassifier("opencode", "glm-5.2", 3, classifiedAt)

	want := domain.Classifier{
		Tool:            "opencode",
		Model:           "glm-5.2",
		TaxonomyVersion: 3,
		ClassifiedAt:    classifiedAt,
	}
	if got.Tool != want.Tool {
		t.Errorf("Tool = %q, want %q", got.Tool, want.Tool)
	}
	if got.Model != want.Model {
		t.Errorf("Model = %q, want %q", got.Model, want.Model)
	}
	if got.TaxonomyVersion != want.TaxonomyVersion {
		t.Errorf("TaxonomyVersion = %d, want %d", got.TaxonomyVersion, want.TaxonomyVersion)
	}
	if !got.ClassifiedAt.Equal(want.ClassifiedAt) {
		t.Errorf("ClassifiedAt = %v, want %v", got.ClassifiedAt, want.ClassifiedAt)
	}
}

// TestNewClassifier_doesNotPickTheModel documents TZ.md section 8.2:
// classify never chooses a model, it only records whatever the caller
// passed in. Two calls with different model ids record different
// models — NewClassifier makes no decision of its own.
func TestNewClassifier_doesNotPickTheModel(t *testing.T) {
	at := time.Now().UTC()

	a := NewClassifier("claude", "opus-4.8", 1, at)
	b := NewClassifier("claude", "sonnet-5", 1, at)

	if a.Model == b.Model {
		t.Fatalf("Model = %q for both calls, want the caller's chosen model to be recorded as given", a.Model)
	}
	if a.Model != "opus-4.8" || b.Model != "sonnet-5" {
		t.Errorf("Model = %q, %q, want %q, %q", a.Model, b.Model, "opus-4.8", "sonnet-5")
	}
}
