package clitree

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// writeResult sends data to cmd's --out file when out is set, or to
// stdout otherwise (TZ.md's "results to stdout (or to --out)" rule).
// Writing to --out is confirmed on stderr, mirroring the cache-hit and
// strategy notices: stdout stays reserved for the result itself.
func writeResult(cmd *cobra.Command, out string, data []byte) error {
	if out == "" {
		_, err := cmd.OutOrStdout().Write(data)
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", out, err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "wrote result to %s\n", out)
	return nil
}

// writeArtifactResult reads back the artifact file a command just wrote
// (or, on a cache hit, the existing one it found) and sends its exact
// bytes to writeResult. Reading the file back, rather than re-rendering
// the in-memory doc, is what keeps this correct on a cache hit: the
// existing file's format is whatever it was originally written in, which
// need not match this run's --format.
func writeArtifactResult(cmd *cobra.Command, artifactPath, out string) error {
	data, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("read %q: %w", artifactPath, err)
	}
	return writeResult(cmd, out, data)
}

// marshalJSONOrYAML renders v as json or yaml -- the only two formats
// available for output that is not one of the four cached artifact kinds
// (get-classify-batch's uncached batch preview, get-stats' aggregate):
// neither has a defined text or html rendering of its own (TZ.md section
// 7.2.1). It marshals to JSON first and, for yaml, decodes that JSON back
// into a generic value before handing it to yaml.Marshal, so both formats
// present the exact same field names and shapes -- including for types
// such as *jsonschema.Schema whose only defined wire encoding is its
// custom MarshalJSON, not a yaml.v3 struct reflection over the same
// fields.
func marshalJSONOrYAML(format artifact.Format, v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	switch format {
	case artifact.FormatJSON:
		return append(data, '\n'), nil
	case artifact.FormatYAML:
		var generic any
		if err := json.Unmarshal(data, &generic); err != nil {
			return nil, fmt.Errorf("decode json for yaml conversion: %w", err)
		}
		out, err := yaml.Marshal(generic)
		if err != nil {
			return nil, fmt.Errorf("marshal yaml: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("format %q: %w", format, artifact.ErrUnsupportedFormat)
	}
}

// reportCacheHit tells the user, on stderr, that a result came from an
// existing artifact without calling GitLab (TZ.md: "a user must be able
// to tell a cached answer from a fresh one").
func reportCacheHit(cmd *cobra.Command, hit bool, path string) {
	if hit {
		fmt.Fprintf(cmd.ErrOrStderr(), "cache hit: %s\n", path)
	}
}

// reportFormatMismatch tells the user, on stderr, when a cache hit's
// existing artifact is not in the requested --format: cache.Lookup only
// ever matches a json or yaml file (TZ.md section 4), so a cache hit
// found while --format asks for a different format (for example text or
// html, or even the other of json/yaml) silently hands back the existing
// file as-is -- writeArtifactResult sends that file's own bytes, in its
// own format, to --out or stdout, and nothing is ever written in the
// requested format. This is the one place --format is not honored on a
// cache hit, so it is reported rather than left silent.
func reportFormatMismatch(cmd *cobra.Command, requested artifact.Format, path string) {
	actual, err := artifact.FormatFromPath(path)
	if err != nil || actual == requested {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"note: --format %s not honored on this cache hit: existing artifact %s is %s\n",
		requested, path, actual)
}

// reportStrategy tells the user, on stderr, which merge request search
// strategy actually ran and what its smoke test found (TZ.md: "It changes
// what the numbers mean and the user never chose it -- the autoselector
// did"), or that point mode ran no candidate search at all when q.MR is
// set.
func reportStrategy(cmd *cobra.Command, q domain.Query) {
	if q.MR != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "mode: point (single merge request, no candidate search)")
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "strategy: %s\nsmoke: %s\n", q.Strategy, q.Smoke)
}
