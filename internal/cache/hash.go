package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
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

// QueryHash returns the artifact cache key for q under toolVersion: the
// Hash of the normalized request fields plus toolVersion, per TZ.md
// section 4.6.
//
// gitlab_url, user_id and tool_version are always present. from/to are
// rendered as YYYY-MM-DD (domain.Date's own wire format). group, project
// and mr are included only when set on q — an omitted optional
// parameter is an absent key, never a null value. Since group/project
// are plain strings on domain.Query, an omitted value and an explicit
// empty string are the same Go zero value and are indistinguishable
// here — both hash as absent, matching domain.Query's own "omitempty"
// tag on those fields. mr is a pointer, so a nil mr (omitted) and a
// non-nil mr pointing at 0 (explicitly set) are distinguishable and
// hash differently.
//
// toolVersion (TZ.md section 4.6): a real incident is why this exists.
// v0.2.0 built its bruteforce list requests without scope=all, so
// GitLab defaulted scope to created_by_me and the search silently came
// back with items: [] for anyone whose own comments were on a
// colleague's merge request. v0.2.1 fixed the request. Without
// toolVersion in this hash, an artifact a v0.2.0 binary had already
// written for a query would still be sitting on disk under the exact
// same "<kind>_<hash>.<ext>" name a v0.2.1 binary computes for the
// identical query — cache.Lookup would call that a fresh hit and hand
// back the empty result for up to a full TTL (24h by default) after the
// fix shipped, with the fix looking like it did nothing. Folding
// toolVersion in here means upgrading past a query-semantics fix always
// changes the cache key, so the next call always misses and refetches
// once, regardless of TTL.
//
// toolVersion is the caller's whole released version (app.ToolVersion),
// not a separate, hand-maintained "did this release change query
// semantics" counter. That is deliberate: a second counter would need
// its own judgment call on every release ("does this one need a bump?")
// on top of the version bump every release already gets — and a missed
// judgment call is exactly the shape of the v0.2.0 bug this guards
// against. The trade-off accepted instead: every release, including one
// that touches nothing about how a query is built, invalidates every
// artifact still inside its TTL once. That is wasted refetching, not a
// correctness risk, and it never requires a second discipline nobody
// will reliably exercise.
//
// Strategy and Smoke are deliberately excluded, even though they are
// fields on domain.Query and end up recorded in the artifact's query
// block for provenance: both fields are outcomes the step decides while
// it runs (TZ.md sections 5.3, 5.5). A cache lookup happens before the
// step runs, so hashing them would make the lookup key depend on a
// result that does not exist yet, and would make the key depend on
// whether the caller happens to look up the cache before or after the
// strategy is resolved.
func QueryHash(q domain.Query, toolVersion string) (string, error) {
	obj := map[string]any{
		"gitlab_url":   q.GitlabURL,
		"user_id":      q.UserID,
		"from":         q.From.String(),
		"to":           q.To.String(),
		"more_than":    q.MoreThan,
		"tool_version": toolVersion,
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

// HashWithToolVersion returns the hex-encoded SHA-256 digest of v's
// canonical JSON encoding, folded together with toolVersion as one more
// top-level key (TZ.md section 4.6) -- the same "add tool_version as a
// sibling key, then hash" shape QueryHash uses for domain.Query,
// generalized to any value that already marshals to a JSON object with
// no fields needing to be excluded.
//
// QueryHash does not call this, and never will: domain.Query carries
// fields (Strategy, Smoke) that must be excluded from the hash because
// they are resolved only after the lookup this hash serves, and one
// field (MR) needs its own nil-vs-explicit-zero rule that a blind
// marshal cannot express -- so QueryHash builds its object by hand
// instead. HashWithToolVersion is for the simpler, more common shape: a
// query type with no such exclusions, where every field is meant to be
// part of the cache key already, and the caller's only gap is that
// tool_version is missing from it. artifact.CommentQuery
// (app.GetComments, get_comments.go) is exactly that shape today.
//
// v must marshal to a JSON object (a struct or map[string]any at the
// top level) -- toolVersion is added as a sibling key, and a value that
// marshals to a JSON array or scalar has no object for that key to join.
// Passing such a v returns an error instead of silently hashing
// something unrelated to it.
func HashWithToolVersion(v any, toolVersion string) (string, error) {
	canonical, err := canonicalJSON(v)
	if err != nil {
		return "", err
	}

	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	var obj map[string]any
	if err := decoder.Decode(&obj); err != nil {
		return "", fmt.Errorf("cache: hash with tool version: value must marshal to a JSON object: %w", err)
	}
	obj["tool_version"] = toolVersion

	return Hash(obj)
}
