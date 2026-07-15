package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// Hash returns the hex-encoded SHA-256 digest of the canonical JSON
// encoding of v (TZ.md section 4.5).
//
// v is round-tripped through a generic JSON value before the final
// marshal: encoding/json always sorts map[string]any keys when it
// marshals a map, regardless of the order the map (or the struct that
// produced it) was built in. That makes the result stable across
// processes and immune to Go's randomized map iteration order. Numbers
// are decoded with json.Number so large ids do not lose precision
// through a float64 round trip.
func Hash(v any) (string, error) {
	canonical, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalJSON renders v as canonical JSON: object keys sorted
// lexicographically, at every nesting level.
func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("cache: marshal value: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, fmt.Errorf("cache: normalize value: %w", err)
	}

	canonical, err := json.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("cache: marshal canonical value: %w", err)
	}
	return canonical, nil
}

// QueryHash returns the artifact cache key for q: the Hash of the
// normalized request fields, per TZ.md section 4.5.
//
// gitlab_url and user_id are always present. from/to are rendered as
// YYYY-MM-DD (domain.Date's own wire format). group, project and mr are
// included only when set on q — an omitted optional parameter is an
// absent key, never a null value. Since group/project are plain strings
// on domain.Query, an omitted value and an explicit empty string are
// the same Go zero value and are indistinguishable here — both hash as
// absent, matching domain.Query's own "omitempty" tag on those fields.
// mr is a pointer, so a nil mr (omitted) and a non-nil mr pointing at 0
// (explicitly set) are distinguishable and hash differently.
//
// Strategy and Smoke are deliberately excluded, even though they are
// fields on domain.Query and end up recorded in the artifact's query
// block for provenance: TZ.md section 4.5 lists only gitlab_url and
// user_id as mandatory members of the hashed object, and both fields
// are outcomes the step decides while it runs (TZ.md sections 5.3,
// 5.5). A cache lookup happens before the step runs, so hashing them
// would make the lookup key depend on a result that does not exist yet,
// and would make the key depend on whether the caller happens to look
// up the cache before or after the strategy is resolved.
func QueryHash(q domain.Query) (string, error) {
	obj := map[string]any{
		"gitlab_url": q.GitlabURL,
		"user_id":    q.UserID,
		"from":       q.From.String(),
		"to":         q.To.String(),
		"more_than":  q.MoreThan,
	}
	if q.Group != "" {
		obj["group"] = q.Group
	}
	if q.Project != "" {
		obj["project"] = q.Project
	}
	if q.MR != nil {
		obj["mr"] = *q.MR
	}
	return Hash(obj)
}
