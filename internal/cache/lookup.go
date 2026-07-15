package cache

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultTTL is the default cache lifetime (TZ.md section 4.5): 24
// hours. Callers (the --cache-ttl flag) override this on Options.TTL.
const DefaultTTL = 24 * time.Hour

// cachedExts lists the extensions Lookup checks, in priority order.
// ExtText is deliberately absent: text is write-only and a lookup must
// never treat a .txt file as a cache hit (TZ.md section 4.5).
var cachedExts = []string{ExtYAML, ExtJSON}

// HeaderReader reads the fetched_at provenance timestamp recorded in an
// existing artifact file at path. cache does not parse the
// text/yaml/json artifact formats itself — that lives in artifact/ —
// so Lookup calls back into whatever does, through this consumer-side
// interface (TZ.md section 2.4).
type HeaderReader interface {
	FetchedAt(path string) (time.Time, error)
}

// Options configures a cache Lookup, and is reused as-is by Get/Put for
// value entries (value.go): both mechanisms share the same Dir/TTL/
// Refresh shape, so one config type is enough for both.
type Options struct {
	// Dir is the artifact directory to search (the --out directory).
	Dir string
	// TTL is how old a cached artifact (or, for Get/Put, a stored
	// value) may be and still count as fresh. Zero effectively
	// disables the cache, since nothing is ever younger than a
	// zero-length TTL. Callers default this to DefaultTTL.
	TTL time.Duration
	// Refresh forces a miss without touching the filesystem, per the
	// --refresh flag (TZ.md section 4.5).
	Refresh bool
}

// Lookup looks for a fresh cached artifact of the given kind and hash
// under opts.Dir (TZ.md section 4.5). It returns hit=true, with the
// artifact path, only when a matching yaml or json file exists and its
// fetched_at header is younger than opts.TTL. A matching .txt file is
// never considered a candidate: text is write-only and is never a
// cache hit, regardless of its age.
func Lookup(kind, hash string, opts Options, headers HeaderReader, now time.Time) (path string, hit bool, err error) {
	if opts.Refresh {
		return "", false, nil
	}

	for _, ext := range cachedExts {
		candidate := filepath.Join(opts.Dir, FileName(kind, hash, ext))

		if _, statErr := os.Stat(candidate); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return "", false, fmt.Errorf("cache lookup: stat %s: %w", candidate, statErr)
		}

		fetchedAt, readErr := headers.FetchedAt(candidate)
		if readErr != nil {
			return "", false, fmt.Errorf("cache lookup: read fetched_at from %s: %w", candidate, readErr)
		}

		if now.Sub(fetchedAt) < opts.TTL {
			return candidate, true, nil
		}
		return "", false, nil
	}

	return "", false, nil
}
