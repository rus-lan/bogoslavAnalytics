package classify

import (
	"errors"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// TestValidate_rejectsLabelOutsideTaxonomy is the acceptance check for
// TZ.md section 8.5 / section 12.10: a label outside the taxonomy is
// rejected.
func TestValidate_rejectsLabelOutsideTaxonomy(t *testing.T) {
	tax := DefaultTaxonomy()
	batch := []domain.Note{sampleNote(1, "looks good"), sampleNote(2, "off by one")}
	result := []NoteLabel{
		{NoteID: 1, Label: "style"},
		{NoteID: 2, Label: "not-a-real-label"},
	}

	_, err := Validate(tax, batch, result)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if len(verr.Problems) != 1 {
		t.Fatalf("Problems = %+v, want exactly 1", verr.Problems)
	}
	if verr.Problems[0].NoteID != 2 {
		t.Errorf("Problems[0].NoteID = %d, want 2", verr.Problems[0].NoteID)
	}
}

// TestValidate_rejectsExtraNoteID is the acceptance check for TZ.md
// section 8.5 / section 12.10: a note_id that was not in the batch is
// rejected.
func TestValidate_rejectsExtraNoteID(t *testing.T) {
	tax := DefaultTaxonomy()
	batch := []domain.Note{sampleNote(1, "looks good"), sampleNote(2, "off by one")}
	result := []NoteLabel{
		{NoteID: 1, Label: "style"},
		{NoteID: 2, Label: "bug"},
		{NoteID: 99, Label: "bug"}, // never part of the batch
	}

	_, err := Validate(tax, batch, result)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if len(verr.Problems) != 1 {
		t.Fatalf("Problems = %+v, want exactly 1", verr.Problems)
	}
	if verr.Problems[0].NoteID != 99 {
		t.Errorf("Problems[0].NoteID = %d, want 99", verr.Problems[0].NoteID)
	}
}

// TestValidate_rejectsMissingNoteID is the acceptance check for TZ.md
// section 8.5 / section 12.10: a note_id from the batch that is missing
// from the result is rejected.
func TestValidate_rejectsMissingNoteID(t *testing.T) {
	tax := DefaultTaxonomy()
	batch := []domain.Note{sampleNote(1, "looks good"), sampleNote(2, "off by one")}
	result := []NoteLabel{
		{NoteID: 1, Label: "style"},
		// note 2 never labeled
	}

	_, err := Validate(tax, batch, result)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if len(verr.Problems) != 1 {
		t.Fatalf("Problems = %+v, want exactly 1", verr.Problems)
	}
	if verr.Problems[0].NoteID != 2 {
		t.Errorf("Problems[0].NoteID = %d, want 2", verr.Problems[0].NoteID)
	}
}

func TestValidate_rejectsDuplicateNoteID(t *testing.T) {
	tax := DefaultTaxonomy()
	batch := []domain.Note{sampleNote(1, "looks good")}
	result := []NoteLabel{
		{NoteID: 1, Label: "style"},
		{NoteID: 1, Label: "bug"},
	}

	_, err := Validate(tax, batch, result)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if len(verr.Problems) != 1 {
		t.Fatalf("Problems = %+v, want exactly 1", verr.Problems)
	}
	if verr.Problems[0].NoteID != 1 {
		t.Errorf("Problems[0].NoteID = %d, want 1", verr.Problems[0].NoteID)
	}
}

// TestValidate_acceptsValidResult is the acceptance check for TZ.md
// section 8.5: a valid result passes and produces []domain.LabeledNote.
func TestValidate_acceptsValidResult(t *testing.T) {
	tax := DefaultTaxonomy()
	note1 := sampleNote(1, "looks good")
	note2 := sampleNote(2, "off by one")
	batch := []domain.Note{note1, note2}
	result := []NoteLabel{
		{NoteID: 1, Label: "praise"},
		{NoteID: 2, Label: "bug"},
	}

	got, err := Validate(tax, batch, result)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	want := []domain.LabeledNote{
		{Note: note1, Label: "praise"},
		{Note: note2, Label: "bug"},
	}
	assertEqualJSON(t, got, want)
}

func TestValidate_multipleProblemsAllReported(t *testing.T) {
	tax := DefaultTaxonomy()
	batch := []domain.Note{sampleNote(1, "a"), sampleNote(2, "b"), sampleNote(3, "c")}
	result := []NoteLabel{
		{NoteID: 1, Label: "not-a-label"}, // bad label
		{NoteID: 99, Label: "bug"},        // extra
		// note 2 and note 3 both missing
	}

	_, err := Validate(tax, batch, result)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error = %v, want *ValidationError", err)
	}
	if len(verr.Problems) != 4 {
		t.Fatalf("Problems = %+v, want exactly 4", verr.Problems)
	}
	if verr.Error() == "" {
		t.Error("ValidationError.Error() = \"\", want a message naming each problem")
	}
}
