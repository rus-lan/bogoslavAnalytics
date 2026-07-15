package app

import (
	"context"
	"fmt"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
)

// resolvedUserCacheName scopes the cache.Get/Put entries ResolveUserCached
// writes: one stored value per {gitlab_url, username} pair (TZ.md sections
// 4.6, 5.0, 14 item 15).
const resolvedUserCacheName = "resolved_user"

// ResolveUser turns a --user value into a numeric GitLab user id: a
// value made only of digits is used as-is, with zero calls to
// resolver.ResolveUserID; anything else is resolved once via
// GET /users?username=... (TZ.md section 5.0).
//
// This is the uncached primitive: it always calls resolver.ResolveUserID
// for a non-numeric user. ResolveUserCached, below, wraps it to add the
// on-disk cache, and every production caller today goes through that
// wrapper instead of calling ResolveUser directly: FindMRs (find_mrs.go)
// and GetComments (get_comments.go) -- the shared implementation behind
// both the find_mrs/find and get_comments/comments MCP tool and CLI
// pairs -- both resolve --user via ResolveUserCached. ResolveUser has no
// caller of its own left; it stays exported as the primitive
// ResolveUserCached is built on.
func ResolveUser(ctx context.Context, resolver UserResolver, user string) (int64, error) {
	if n, ok := parseNumericID(user); ok {
		return n, nil
	}
	id, err := resolver.ResolveUserID(ctx, user)
	if err != nil {
		return 0, fmt.Errorf("app: resolve user %q: %w", user, err)
	}
	return id, nil
}

// ResolveUserCached is ResolveUser plus an on-disk cache keyed on
// {gitlab_url, user}, using the cache.Options/now shape find_mrs.go
// already builds for the artifact-1 lookup (TZ.md section 4.6). A numeric
// user is still handled with zero calls to resolver.ResolveUserID and
// never touches the cache at all -- the same short-circuit ResolveUser
// applies.
//
// gitlab_url is part of the key on purpose: the same username on two
// different GitLab instances names two different people, so a hit on one
// instance must never answer for the other.
//
// THE RENAME HAZARD -- read this before raising this cache's TTL or
// reusing it somewhere new (TZ.md sections 4.6, 5.0, 14 item 15):
//
// GitLab usernames can be changed, and once a username is changed the old
// name becomes free for a *different* person to claim. A cached
// "alice -> 42" mapping that survives past the moment "alice" is renamed
// and re-claimed by someone else means every hit against that entry, for
// the rest of its TTL, silently reports the new claimant's merge request
// activity under the name the caller asked about -- there is no error,
// no warning, because a resolved numeric id is all any downstream code
// ever compares notes[].author.id against. This is a sharper version of
// the group/project rename hazard TZ.md section 4.6 already accepts (see
// also FindMRsRequest's doc comment): there, a stale cache key answers
// with the wrong *selection* of merge requests; here, it answers with the
// wrong *human* entirely. It is bounded the same way that hazard is --
// by opts.TTL (default cache.DefaultTTL, 24h) -- and escapable the same
// way -- the --refresh flag forces a miss -- but neither of those fixes
// it, they only bound how long it can go unnoticed.
func ResolveUserCached(ctx context.Context, resolver UserResolver, gitlabURL, user string, opts cache.Options, now time.Time) (int64, error) {
	if n, ok := parseNumericID(user); ok {
		return n, nil
	}

	hash, hashErr := cache.Hash(map[string]any{
		"gitlab_url": gitlabURL,
		"username":   user,
	})
	if hashErr == nil {
		if id, hit := cache.Get[int64](resolvedUserCacheName, hash, opts, now); hit {
			return id, nil
		}
	}

	id, err := ResolveUser(ctx, resolver, user)
	if err != nil {
		return 0, err
	}

	if hashErr == nil {
		// Best-effort: the resolution above already succeeded, so a
		// failure to persist it only costs the next call one extra
		// GitLab round trip -- it never changes this call's result.
		_ = cache.Put(resolvedUserCacheName, hash, id, opts, now)
	}
	return id, nil
}
