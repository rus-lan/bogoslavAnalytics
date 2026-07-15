package cache

import (
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// LabelKey identifies a cached labeling run for a batch of comments: it
// changes if the batch's comments change, or if the model, or if the
// taxonomy version changes (TZ.md section 8.4). It is comparable with
// ==, so two LabelKey values built at different times can be compared
// directly to decide whether get_classify_batch may reuse an existing
// labeled_comments artifact instead of handing out a batch to relabel.
type LabelKey struct {
	ItemsHash       string
	Model           string
	TaxonomyVersion int
}

// NewLabelKey builds the labeling cache key for items labeled with
// model under taxonomyVersion (TZ.md section 8.4).
func NewLabelKey(items []domain.Note, model string, taxonomyVersion int) (LabelKey, error) {
	itemsHash, err := Hash(items)
	if err != nil {
		return LabelKey{}, fmt.Errorf("label key: hash items: %w", err)
	}
	return LabelKey{
		ItemsHash:       itemsHash,
		Model:           model,
		TaxonomyVersion: taxonomyVersion,
	}, nil
}
