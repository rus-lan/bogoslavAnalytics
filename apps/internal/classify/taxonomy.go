package classify

import (
	"encoding/json"
	"fmt"
)

// LabelOther is the required fallback label. The validator must always
// have a valid target for a comment that does not fit any other label,
// so every taxonomy must include it (TZ.md section 8.5).
const LabelOther = "other"

// DefaultTaxonomyVersion is the taxonomy_version of DefaultTaxonomy.
const DefaultTaxonomyVersion = 1

// Taxonomy is a versioned set of labels the calling agent may assign to
// a comment. It ships as data (TZ.md section 8.5): either the default
// v1 set returned by DefaultTaxonomy, or a user-edited set built with
// NewTaxonomy. Changing the label set — adding, removing, or renaming a
// label — must bump Version, so the classifier provenance recorded on
// an artifact-3 names the exact rules that were applied.
type Taxonomy struct {
	Version int      `json:"version"`
	Labels  []string `json:"labels"`
}

// DefaultTaxonomy returns the v1 label set shipped out of the box for
// code-review comments (TZ.md section 8.5).
func DefaultTaxonomy() Taxonomy {
	t, err := NewTaxonomy(DefaultTaxonomyVersion, []string{
		"bug", "style", "naming", "architecture", "performance",
		"security", "test", "docs", "question", "nitpick", "praise",
		LabelOther,
	})
	if err != nil {
		panic(fmt.Sprintf("classify: default taxonomy is invalid: %v", err))
	}
	return t
}

// NewTaxonomy builds a Taxonomy from a version and a label set, for
// example when loading a user-edited taxonomy. It rejects a set that
// omits the required fallback label "other" (TZ.md section 8.5).
func NewTaxonomy(version int, labels []string) (Taxonomy, error) {
	t := Taxonomy{Version: version, Labels: labels}
	if !t.Has(LabelOther) {
		return Taxonomy{}, fmt.Errorf("taxonomy version %d: %w", version, ErrTaxonomyMissingOther)
	}
	return t, nil
}

// Has reports whether label is a member of the taxonomy.
func (t Taxonomy) Has(label string) bool {
	for _, l := range t.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// UnmarshalJSON decodes a taxonomy and rejects one that omits the
// required fallback label "other" (TZ.md section 8.5): loading a
// user-supplied taxonomy must reject such a set.
func (t *Taxonomy) UnmarshalJSON(data []byte) error {
	type shape Taxonomy
	var s shape
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	built, err := NewTaxonomy(s.Version, s.Labels)
	if err != nil {
		return err
	}
	*t = built
	return nil
}
