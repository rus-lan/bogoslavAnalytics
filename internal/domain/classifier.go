package domain

import "time"

// Classifier is the provenance block recorded on a labeled_comments
// artifact: who/what performed the labeling and with which taxonomy
// version. It is mandatory on save_labels (TZ.md section 8.3).
type Classifier struct {
	Tool            string    `json:"tool"`
	Model           string    `json:"model"`
	TaxonomyVersion int       `json:"taxonomy_version"`
	ClassifiedAt    time.Time `json:"classified_at"`
}
