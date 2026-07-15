package app

import (
	"context"
	"fmt"
)

// ResolveUser turns a --user value into a numeric GitLab user id: a
// value made only of digits is used as-is, with zero calls to
// resolver.ResolveUserID; anything else is resolved once via
// GET /users?username=... (TZ.md section 5.0). Any use case, or a
// CLI/MCP wrapper acting ahead of one, calls this once per pipeline run
// before building a request that carries a resolved user id, so username
// resolution never happens twice for the same --user value.
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
