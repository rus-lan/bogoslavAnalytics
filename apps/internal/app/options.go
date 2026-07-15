package app

import (
	"path/filepath"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/cache"
)

// defaultArtifactDir is where artifacts land when a request leaves Dir
// empty, relative to the working directory of the calling process
// (TZ.md section 3), overridden by the --out flag on the calling
// command/tool.
const defaultArtifactDir = "artifacts"

// CacheOptions carries the --cache-ttl/--refresh pair TZ.md section 4.5
// puts on find_mrs: TTL zero means cache.DefaultTTL (24h); Refresh
// forces a miss without touching the filesystem.
type CacheOptions struct {
	TTL     time.Duration
	Refresh bool
}

// ttl returns o.TTL, or cache.DefaultTTL if it is not set.
func (o CacheOptions) ttl() time.Duration {
	if o.TTL > 0 {
		return o.TTL
	}
	return cache.DefaultTTL
}

// outDir returns dir, or defaultArtifactDir if dir is empty.
func outDir(dir string) string {
	if dir == "" {
		return defaultArtifactDir
	}
	return dir
}

// outFormat returns format, or artifact.FormatYAML if format is empty
// (TZ.md section 4: "--format, дефолт yaml").
func outFormat(format artifact.Format) artifact.Format {
	if format == "" {
		return artifact.FormatYAML
	}
	return format
}

// clockOrDefault returns now, or time.Now if now is nil.
func clockOrDefault(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}
	return time.Now
}

// artifactPath builds the "<kind>_<hash>.<ext>" path (TZ.md section 4.5)
// under dir for the given kind, hash and format.
func artifactPath(dir string, kind artifact.Kind, hash string, format artifact.Format) (string, error) {
	ext, err := format.Extension()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cache.FileName(string(kind), hash, ext)), nil
}
