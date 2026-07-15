package app

import (
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/classify"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// notesOf extracts the domain.Note batch out of a comment_list
// artifact's rows, in the same order as items, for classify.Validate and
// the labeling cache key (TZ.md section 8.4).
func notesOf(items []artifact.CommentItem) []domain.Note {
	notes := make([]domain.Note, len(items))
	for i, it := range items {
		notes[i] = it.Note
	}
	return notes
}

// resolveTaxonomy returns *t, or classify.DefaultTaxonomy() if t is nil
// (TZ.md section 8.5).
func resolveTaxonomy(t *classify.Taxonomy) classify.Taxonomy {
	if t != nil {
		return *t
	}
	return classify.DefaultTaxonomy()
}

// labelArtifactHash returns the labeling cache key hash (TZ.md section
// 8.4) for a batch of notes labeled with model under taxonomyVersion.
// GetClassifyBatch and SaveLabels both call this with the exact same
// inputs (the same comment_list's notes, the same model, the same
// taxonomy version), so they derive the exact same hash and therefore
// agree on the same "<kind>_<hash>.<ext>" file name: SaveLabels writes
// it, and a later GetClassifyBatch call's cache check looks for it.
func labelArtifactHash(notes []domain.Note, model string, taxonomyVersion int) (string, error) {
	key, err := cache.NewLabelKey(notes, model, taxonomyVersion)
	if err != nil {
		return "", fmt.Errorf("label key: %w", err)
	}
	hash, err := cache.Hash(key)
	if err != nil {
		return "", fmt.Errorf("hash label key: %w", err)
	}
	return hash, nil
}
