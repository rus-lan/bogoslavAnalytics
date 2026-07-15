package classify

import "errors"

// ErrTaxonomyMissingOther is returned by NewTaxonomy, and by
// Taxonomy.UnmarshalJSON, when a label set omits the required fallback
// label "other" (TZ.md section 8.5).
var ErrTaxonomyMissingOther = errors.New("classify: taxonomy is missing the required fallback label \"other\"")
