package app

import "net/url"

// sanitizeGitlabURL strips any userinfo (user:pass@ or user@) from raw
// before the value is used for anything except the real GitLab HTTP
// request.
//
// GitlabURL flows to two different places, with two different rules:
//
//   - The actual request: gitlab.Client is built straight from the raw
//     GITLAB_URL (gitlab.NewClientFromEnv), never through this
//     function, so a real "https://oauth2:token@host" URL -- GitLab's
//     own documented idiom -- keeps authenticating exactly as before.
//   - Provenance and cache keys: every FindMRs/GetComments use of
//     req.GitlabURL below this point -- artifact.Source.GitlabURL,
//     domain.Query.GitlabURL, and the {gitlab_url, ...} cache keys
//     ResolveUserCached and cachingSmokeClient build -- only records
//     or hashes which instance was talked to. It never needs, and must
//     never carry, credentials: an artifact is meant to be read by a
//     person or handed to another tool, not used to authenticate.
//
// This is the one seam both bogoslav-cli and bogoslav-mcp funnel
// through (FindMRs/GetComments are the shared implementation behind
// both the CLI commands and the MCP tools), so sanitizing here fixes
// both callers at once.
//
// Stripping userinfo here changes the {gitlab_url, ...} cache keys
// computed from the sanitized value (cache.QueryHash, and the
// resolved-user/smoke-test caches in user.go and smoke_cache.go): a
// request made before this fix and one made after it, both against the
// same credentialed GITLAB_URL, no longer hash to the same key. That is
// accepted, not a regression to guard against -- a stale entry simply
// misses once and gets rewritten under the new, credential-free key.
//
// A URL that fails to parse, or has no userinfo, is returned unchanged.
func sanitizeGitlabURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}
