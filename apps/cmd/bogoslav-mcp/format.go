package main

import (
	"fmt"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
)

// parseFormat validates a tool's format input against the four artifact
// wire formats (TZ.md section 4). An empty string is valid and passed
// through unchanged: it means "not set", and every app.XxxRequest that
// carries a Format field already defaults an empty value to
// artifact.FormatYAML itself (app.outFormat) -- this function only
// rejects a value that is neither empty nor one of the four.
func parseFormat(s string) (artifact.Format, error) {
	switch f := artifact.Format(s); f {
	case "", artifact.FormatJSON, artifact.FormatYAML, artifact.FormatText, artifact.FormatHTML:
		return f, nil
	default:
		return "", fmt.Errorf("format %q: must be one of json, yaml, text, html", s)
	}
}

// cacheOptions converts a tool's refresh/cache_ttl_seconds pair into an
// app.CacheOptions. A ttlSeconds of zero or less leaves TTL unset, so
// app.CacheOptions.ttl() falls back to cache.DefaultTTL (24h) exactly as
// bogoslav-cli's own --cache-ttl default does.
func cacheOptions(refresh bool, ttlSeconds int64) app.CacheOptions {
	var ttl time.Duration
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	return app.CacheOptions{TTL: ttl, Refresh: refresh}
}
