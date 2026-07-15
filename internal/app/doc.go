// Package app is the shared use-case layer TZ.md section 7.3 requires:
// bogoslav-cli and bogoslav-mcp must be thin adapters over the exact same
// six functions here (FindMRs, GetComments, GetClassifyBatch, SaveLabels,
// FilterComments, GetStats) -- one function per command/tool pair -- so
// the CLI and the MCP tools can never drift into two implementations of
// the same use case.
//
// app composes internal/{domain,gitlab,search,artifact,cache,filter,
// classify,stats}. It never talks to GitLab's HTTP API directly (that
// stays in gitlab/, reached only through the consumer-side interfaces
// this package declares for itself, TZ.md section 2.4) and it never
// calls an LLM (TZ.md section 8.1: the calling agent labels, classify/
// only owns the contract).
//
// Cache key hazard (TZ.md section 4.6, accepted, not fixed here):
// cache.QueryHash, which FindMRs uses to name and look up artifact-1,
// hashes the query's Group/Project as PATH strings, not resolved
// numeric ids. If a group or project is renamed and a new one takes
// over the old path, the same cache key will answer for the new
// object's data until the cached entry ages out. This is bounded by the
// default 24h TTL (cache.DefaultTTL) and is an accepted risk, not a bug
// this package works around.
//
// A second cache hazard, sharper than the one above: ResolveUserCached
// (user.go) caches a username's resolved numeric id keyed on
// {gitlab_url, username}. GitLab usernames can be renamed and the freed
// name later re-claimed by someone else; a cache entry that survives
// past that moment answers with the wrong *person*, not just the wrong
// *selection* the hazard above describes. See ResolveUserCached's doc
// comment for the full explanation. Bounded by the same TTL, escaped by
// the same --refresh flag, not fixed here either.
package app
