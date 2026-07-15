package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
)

// commonOutputFlags are the three flags every command in this tree
// carries (TZ.md section 3, section 4): --format chooses the artifact
// wire format, --artifacts-dir is the directory a command's artifact is
// written under (and, for json/yaml, looked up in as a cache), and --out
// redirects the copy of the result this command prints away from stdout
// and into a file instead.
type commonOutputFlags struct {
	format string
	dir    string
	out    string
}

// addCommonOutputFlags registers --format, --artifacts-dir and --out on
// cmd, storing their values in f.
func addCommonOutputFlags(cmd *cobra.Command, f *commonOutputFlags) {
	cmd.Flags().StringVar(&f.format, "format", string(artifact.FormatYAML),
		"output format: json, yaml, text, or html. text and html are write-only: "+
			"they cannot be read back and cannot be passed to --from-artifact, and they are never a cache hit")
	cmd.Flags().StringVar(&f.dir, "artifacts-dir", "",
		`directory the result is written under, and where a matching json/yaml artifact is looked up as a cache (default "artifacts")`)
	cmd.Flags().StringVar(&f.out, "out", "",
		"write the result to this file instead of stdout")
}

// cacheFlags are --refresh and --cache-ttl, wired only on the two
// commands whose app request carries app.CacheOptions (find-mrs,
// get-comments): every other command either never consults a cache
// (save-labels, filter-comments, get-stats) or has no adjustable TTL of
// its own (get-classify-batch), so adding these flags there would be
// dishonest -- a flag that looks like it does something but never does.
type cacheFlags struct {
	refresh bool
	ttl     time.Duration
}

// addCacheFlags registers --refresh and --cache-ttl on cmd, storing their
// values in f.
func addCacheFlags(cmd *cobra.Command, f *cacheFlags) {
	cmd.Flags().BoolVar(&f.refresh, "refresh", false,
		"bypass the cache and always call GitLab, even if a fresh cached artifact exists")
	cmd.Flags().DurationVar(&f.ttl, "cache-ttl", 0,
		`how long a cached artifact stays fresh, for example "24h" (default 24h)`)
}

// parseFormat validates s against the four artifact wire formats
// (TZ.md section 4). Every command runs its --format value through this
// before anything else, so an unknown value fails fast and clearly.
func parseFormat(s string) (artifact.Format, error) {
	switch f := artifact.Format(s); f {
	case artifact.FormatJSON, artifact.FormatYAML, artifact.FormatText, artifact.FormatHTML:
		return f, nil
	default:
		return "", fmt.Errorf("--format %q: must be one of json, yaml, text, html", s)
	}
}
