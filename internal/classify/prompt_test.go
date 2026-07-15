package classify

import (
	"strings"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func TestRenderPrompt_includesTaxonomyAndBatch(t *testing.T) {
	tax := DefaultTaxonomy()
	notes := []domain.Note{sampleNote(1, "please rename this variable")}

	got, err := RenderPrompt(PromptData{Taxonomy: tax, Notes: notes})
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}

	for _, label := range tax.Labels {
		if !strings.Contains(got, label) {
			t.Errorf("prompt does not mention taxonomy label %q:\n%s", label, got)
		}
	}
	if !strings.Contains(got, "please rename this variable") {
		t.Errorf("prompt does not mention the comment body:\n%s", got)
	}
	if !strings.Contains(got, "note_id 1") {
		t.Errorf("prompt does not mention note_id 1:\n%s", got)
	}
}

func TestRenderPrompt_emptyBatch(t *testing.T) {
	got, err := RenderPrompt(PromptData{Taxonomy: DefaultTaxonomy(), Notes: nil})
	if err != nil {
		t.Fatalf("RenderPrompt() error = %v", err)
	}
	if got == "" {
		t.Error("RenderPrompt() = \"\", want the template text even with no notes")
	}
}
