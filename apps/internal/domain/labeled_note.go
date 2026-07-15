package domain

// LabeledNote is a Note carrying the semantic label assigned to it by the
// calling agent during classification (TZ.md section 4.3).
type LabeledNote struct {
	Note
	Label string `json:"label"`
}
