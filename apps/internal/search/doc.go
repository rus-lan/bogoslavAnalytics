// Package search implements the two merge request search strategies of
// TZ.md section 5 -- events (primary) and bruteforce (fallback) -- the
// strategy autoselector, and the exact per-merge-request comment count
// both strategies rely on.
//
// search does not talk HTTP itself. It defines its own consumer-side
// Client interface for the handful of gitlab/ methods it needs, per
// TZ.md section 2.4, so its strategies can be tested against a fake
// instead of a real GitLab instance. *gitlab.Client satisfies that
// interface, but no function in this package ever names the concrete
// *gitlab.Client type.
//
// Resolving --user (username or id) to a numeric id, and routing the
// point mode of TZ.md section 7.2, are the caller's job. A --group or
// --project path needs no such resolution: Params.Scope carries a
// gitlab.ID, which goes straight into gitlab/'s :id parameters whether it
// holds a numeric id or a namespaced path.
package search
