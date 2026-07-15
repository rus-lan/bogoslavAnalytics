package domain

// Discussion is a GitLab discussion thread, as returned by
// GET /projects/:id/merge_requests/:iid/discussions (TZ.md section 4.2).
//
// Its ID is a string (SHA-like) and must not be confused with the
// integer ID carried by each entry in Notes.
type Discussion struct {
	ID             string `json:"id"`
	IndividualNote bool   `json:"individual_note"`
	Notes          []Note `json:"notes"`
}
