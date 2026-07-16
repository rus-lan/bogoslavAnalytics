package app

import (
	"context"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/search"
)

// smokeCacheName scopes the cache.Get/Put entries cachingSmokeClient
// writes: one stored value per {gitlab_url, user_id} pair (TZ.md sections
// 5.5, 14 item 15).
const smokeCacheName = "smoke_test"

// cachingSmokeClient wraps a search.Client so that search.SelectStrategy's
// one call to SmokeTest (TZ.md section 5.3b) is served from an on-disk
// cache when a fresh entry exists, and refreshes that entry on a miss.
// Every other search.Client method is promoted straight through to the
// embedded Client, untouched -- this type exists to intercept exactly one
// method, nothing else.
//
// This is deliberately the only change here: search.SelectStrategy stays
// the one and only place that turns a smoke result into a strategy choice
// (retention exceeded / smoke not passed / --strict, all TZ.md section
// 5.3). cachingSmokeClient never inspects or repeats that decision -- it
// only decides whether SelectStrategy's call to SmokeTest has to reach
// GitLab to get its answer, or can read a still-fresh answer off disk.
// Duplicating any of section 5.3's rules here would risk the two copies
// drifting apart; wrapping the client keeps there from ever being a
// second copy.
//
// Keyed on {gitlab_url, user_id, tool_version}, deliberately not on
// gitlab_url alone: the smoke probe needs a user who actually has
// thread replies to test against (TZ.md section 5.5.4), so on one
// instance it can legitimately come back SmokeUnknown for a user with
// no such replies while coming back SmokePassed for a different user.
// SmokeUnknown is the autoselector's conservative fallback -- it forces
// bruteforce (TZ.md section 5.3b). Keying on the instance alone would
// let one user's inconclusive probe silently force bruteforce (roughly
// 10x the API call volume, TZ.md section 5) onto every other user
// asking about the same instance, for a reason that has nothing to do
// with them.
//
// tool_version is folded in for the same reason cache.QueryHash folds
// it into artifact-1's key (TZ.md section 4.6), not merely by analogy:
// TZ.md section 5.5 documents a REAL prior incident in this exact
// probe -- the first SmokeTest implementation sampled only 5 candidates
// and skipped any with zero DiscussionNote replies before comparing
// counts, which produced wrong verdicts on live data and was rewritten
// (the procedure this file's SmokeTest call now runs). A cached
// SmokeResult is this probe's own interpretation of raw event/discussion
// counts, computed by OUR heuristic -- unlike ResolveUserCached's
// {gitlab_url, username} cache (user.go), which stores a plain fact
// GitLab itself owns (a username's numeric id) and deliberately does
// NOT fold in tool_version, see that function's doc comment. If a
// future release changes this probe's heuristic again (candidate
// selection, the comparable-candidate rule, the budget, anything in
// TZ.md section 5.5's procedure), a smoke_test entry a pre-fix binary
// already wrote must not go on answering a post-fix binary's identical
// {gitlab_url, user_id} query for the rest of its TTL -- that would
// silently defeat the fix in exactly the shape TZ.md section 4.6's
// v0.2.0 incident already demonstrated once, one layer removed (a
// stale verdict picking the wrong search strategy, instead of a stale
// artifact answering with the wrong items).
type cachingSmokeClient struct {
	search.Client
	gitlabURL string
	opts      cache.Options
	now       func() time.Time
}

var _ search.Client = (*cachingSmokeClient)(nil)

// SmokeTest overrides the promoted search.Client.SmokeTest. A fresh cache
// hit answers without calling through to the embedded Client; every other
// outcome -- opts.Refresh set, no entry, an expired entry, a malformed
// entry, a hash failure -- falls through to a real probe and, on success,
// refreshes the cache entry for next time. A failure to read or write the
// cache entry is never surfaced as an error here: the worst outcome is
// one extra events-plus-discussions round trip, matching cache.Get/Put's
// own documented contract (this cache is a performance optimization, not
// a source of truth).
func (c *cachingSmokeClient) SmokeTest(ctx context.Context, userID int64) (domain.SmokeResult, error) {
	hash, hashErr := cache.Hash(map[string]any{
		"gitlab_url":   c.gitlabURL,
		"user_id":      userID,
		"tool_version": ToolVersion,
	})
	if hashErr == nil {
		if cached, hit := cache.Get[domain.SmokeResult](smokeCacheName, hash, c.opts, c.now()); hit {
			return cached, nil
		}
	}

	result, err := c.Client.SmokeTest(ctx, userID)
	if err != nil {
		return "", err
	}

	if hashErr == nil {
		// Best-effort, for the same reason ResolveUserCached's Put is
		// (user.go): the probe above already succeeded, so a failure to
		// persist it only costs the next call one extra round trip.
		_ = cache.Put(smokeCacheName, hash, result, c.opts, c.now())
	}
	return result, nil
}
