package classify

import (
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// NewClassifier builds the classifier provenance block recorded on a
// labeled_comments artifact: which tool ran the labeling, which model
// it used, which taxonomy version it labeled against, and when (TZ.md
// section 8.3). classify never chooses a model and never calls one
// (TZ.md section 8.2): tool and model are supplied by the calling
// agent and only recorded here, for provenance and for the labeling
// cache key (TZ.md section 8.4).
func NewClassifier(tool, model string, taxonomyVersion int, classifiedAt time.Time) domain.Classifier {
	return domain.Classifier{
		Tool:            tool,
		Model:           model,
		TaxonomyVersion: taxonomyVersion,
		ClassifiedAt:    classifiedAt,
	}
}
