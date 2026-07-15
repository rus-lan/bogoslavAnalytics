package domain

import (
	"encoding/json"
	"time"
)

// NoteType is the GitLab note "type" attribute. On the wire it is either
// the string "DiscussionNote", the string "DiffNote", or JSON null for a
// plain, non-threaded comment. NoteTypeNone (the zero value) models that
// null case — see TZ.md section 4.2.
type NoteType string

const (
	// NoteTypeNone is a plain, non-threaded comment. It round-trips to
	// and from JSON null, never to an empty JSON string.
	NoteTypeNone NoteType = ""
	// NoteTypeDiscussion is a reply inside a discussion thread.
	NoteTypeDiscussion NoteType = "DiscussionNote"
	// NoteTypeDiff is a comment attached to a specific diff line.
	NoteTypeDiff NoteType = "DiffNote"
)

// MarshalJSON renders NoteTypeNone as JSON null and any other value as a
// JSON string.
func (t NoteType) MarshalJSON() ([]byte, error) {
	if t == NoteTypeNone {
		return []byte("null"), nil
	}
	return json.Marshal(string(t))
}

// UnmarshalJSON accepts JSON null (mapped to NoteTypeNone) or a JSON
// string.
func (t *NoteType) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*t = NoteTypeNone
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*t = NoteType(s)
	return nil
}

// NotePosition is the diff position carried by DiffNote records. Field
// names follow the documented attributes in TZ.md section 4.2. LineRange
// is kept as raw JSON: its own inner shape is not part of the verified
// contract, so it is passed through rather than guessed at.
type NotePosition struct {
	BaseSHA      string          `json:"base_sha,omitempty"`
	StartSHA     string          `json:"start_sha,omitempty"`
	HeadSHA      string          `json:"head_sha,omitempty"`
	OldPath      string          `json:"old_path,omitempty"`
	NewPath      string          `json:"new_path,omitempty"`
	PositionType string          `json:"position_type,omitempty"`
	OldLine      int             `json:"old_line,omitempty"`
	NewLine      int             `json:"new_line,omitempty"`
	LineRange    json.RawMessage `json:"line_range,omitempty"`
}

// Note is a single GitLab discussion note, as returned inside
// notes[] by GET /projects/:id/merge_requests/:iid/discussions
// (TZ.md section 4.2). Unlike the Note API, this shape is safe to trust
// for DiscussionNote records.
type Note struct {
	ID           int64      `json:"id"`
	Type         NoteType   `json:"type"`
	Body         string     `json:"body"`
	Author       Author     `json:"author"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at,omitzero"`
	System       bool       `json:"system"`
	NoteableID   int64      `json:"noteable_id"`
	NoteableType string     `json:"noteable_type"`
	ProjectID    int64      `json:"project_id"`
	Resolved     bool       `json:"resolved,omitempty"`
	Resolvable   bool       `json:"resolvable,omitempty"`
	ResolvedBy   *Author    `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	// CommitID and Position are set only on DiffNote records.
	CommitID string        `json:"commit_id,omitempty"`
	Position *NotePosition `json:"position,omitempty"`
}
