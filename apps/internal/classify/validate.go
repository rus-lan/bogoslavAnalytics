package classify

import (
	"fmt"
	"strings"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

// Problem is a single reason a labeling result failed validation: which
// note it concerns, and what is wrong with it.
type Problem struct {
	NoteID int64
	Reason string
}

// Error renders the problem as "note <id>: <reason>".
func (p Problem) Error() string {
	return fmt.Sprintf("note %d: %s", p.NoteID, p.Reason)
}

// ValidationError collects every Problem found while validating a
// labeling result, so a caller such as save_labels can report every
// violation in one pass instead of stopping at the first (TZ.md section
// 7.2). It is never returned empty.
type ValidationError struct {
	Problems []Problem
}

// Error renders every problem, joined into one message.
func (e *ValidationError) Error() string {
	reasons := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		reasons[i] = p.Error()
	}
	return fmt.Sprintf("classify: labeling result rejected (%d problem(s)): %s", len(e.Problems), strings.Join(reasons, "; "))
}

// Validate checks a labeling result against taxonomy and the batch of
// notes it was produced for, and on success pairs every note with its
// label into a []domain.LabeledNote, in batch order (TZ.md section
// 8.5). It rejects, collecting every violation rather than stopping at
// the first:
//
//   - a label that is not a member of taxonomy
//   - a note_id in result that was not in batch (extra)
//   - a note_id in batch that has no entry in result (missing)
//   - a note_id labeled more than once (duplicate)
//
// A rejected result must not reach an artifact (TZ.md section 8.1): the
// caller must check the returned error before writing anything.
func Validate(taxonomy Taxonomy, batch []domain.Note, result []NoteLabel) ([]domain.LabeledNote, error) {
	notesByID := make(map[int64]domain.Note, len(batch))
	for _, note := range batch {
		notesByID[note.ID] = note
	}

	var problems []Problem
	// attempted tracks every batch note_id the result addressed, valid
	// or not, so a note with an invalid label is reported once (as a
	// bad label) rather than twice (as a bad label and, again, as
	// missing from the result).
	attempted := make(map[int64]bool, len(result))
	labelByID := make(map[int64]string, len(result))

	for _, entry := range result {
		if _, inBatch := notesByID[entry.NoteID]; !inBatch {
			problems = append(problems, Problem{NoteID: entry.NoteID, Reason: "note_id is not part of the batch"})
			continue
		}
		if attempted[entry.NoteID] {
			problems = append(problems, Problem{NoteID: entry.NoteID, Reason: "note_id is labeled more than once"})
			continue
		}
		attempted[entry.NoteID] = true

		if !taxonomy.Has(entry.Label) {
			problems = append(problems, Problem{NoteID: entry.NoteID, Reason: fmt.Sprintf("label %q is not in the taxonomy", entry.Label)})
			continue
		}
		labelByID[entry.NoteID] = entry.Label
	}

	for _, note := range batch {
		if !attempted[note.ID] {
			problems = append(problems, Problem{NoteID: note.ID, Reason: "note_id from the batch is missing from the labeling result"})
		}
	}

	if len(problems) > 0 {
		return nil, &ValidationError{Problems: problems}
	}

	labeled := make([]domain.LabeledNote, 0, len(batch))
	for _, note := range batch {
		labeled = append(labeled, domain.LabeledNote{Note: note, Label: labelByID[note.ID]})
	}
	return labeled, nil
}
