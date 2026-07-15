package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/stats"
)

// GetStatsRequest is the input to GetStats, mirroring the get_stats tool
// / stats command surface (TZ.md section 7.2). GetStats never re-reads
// GitLab: it only aggregates the items an already-written artifact
// carries, whichever of the four kinds ArtifactPath turns out to be.
type GetStatsRequest struct {
	ArtifactPath string
	// Dir, when set, writes the aggregate to a file under it; ""
	// (the default) returns the summary without writing (TZ.md section
	// 7.2.1: "без записи файла, если не передан out"). Only json/yaml
	// are supported for the write: stats.Stats is not one of the four
	// artifact kinds, carries no schema_version/source/query, and has no
	// text/html rendering of its own (TZ.md section 7.2.1).
	Dir    string
	Format artifact.Format
}

// GetStatsResult is the output of GetStats: the aggregate, and the path
// it was written to (empty when Dir was not set).
type GetStatsResult struct {
	Stats stats.Stats
	Path  string
}

// GetStats is the shared implementation behind the get_stats MCP tool
// and the stats CLI command (TZ.md section 7.2): detect which of the
// four artifact kinds ArtifactPath holds and aggregate it via stats/.
func GetStats(req GetStatsRequest) (GetStatsResult, error) {
	s, err := readStats(req.ArtifactPath)
	if err != nil {
		return GetStatsResult{}, fmt.Errorf("get stats: %w", err)
	}

	result := GetStatsResult{Stats: s}
	if req.Dir == "" {
		return result, nil
	}

	path, err := writeStats(s, outFormat(req.Format), req.Dir, req.ArtifactPath)
	if err != nil {
		return GetStatsResult{}, fmt.Errorf("get stats: %w", err)
	}
	result.Path = path
	return result, nil
}

// readStats decodes path as whichever of the four artifact kinds it
// turns out to hold, trying each ReadX function in turn: a kind
// mismatch (artifact.ErrKindMismatch) means try the next kind; any other
// error (a missing file, a decode failure, an unknown schema version) is
// real and stops the search immediately, since it is not specific to the
// kind that was just guessed.
func readStats(path string) (stats.Stats, error) {
	if doc, err := artifact.ReadMRList(path); err == nil {
		return stats.FromMRList(doc), nil
	} else if !errors.Is(err, artifact.ErrKindMismatch) {
		return stats.Stats{}, fmt.Errorf("read %q: %w", path, err)
	}

	if doc, err := artifact.ReadCommentList(path); err == nil {
		return stats.FromCommentList(doc), nil
	} else if !errors.Is(err, artifact.ErrKindMismatch) {
		return stats.Stats{}, fmt.Errorf("read %q: %w", path, err)
	}

	if doc, err := artifact.ReadLabeledComments(path); err == nil {
		return stats.FromLabeledComments(doc), nil
	} else if !errors.Is(err, artifact.ErrKindMismatch) {
		return stats.Stats{}, fmt.Errorf("read %q: %w", path, err)
	}

	doc, err := artifact.ReadFilteredComments(path)
	if err != nil {
		return stats.Stats{}, fmt.Errorf("read %q: %w", path, err)
	}
	return stats.FromFilteredComments(doc), nil
}

// writeStats renders s as json or yaml (text/html are not supported: see
// GetStatsRequest's doc comment) and writes it to
// "<dir>/stats_<source artifact base name>.<ext>".
func writeStats(s stats.Stats, format artifact.Format, dir, sourcePath string) (string, error) {
	data, ext, err := marshalStats(s, format)
	if err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	path := filepath.Join(dir, "stats_"+base+"."+ext)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create directory %q: %w", dir, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write file %q: %w", path, err)
	}
	return path, nil
}

// marshalStats renders s as json or yaml, always through JSON first: s
// carries only json struct tags, so calling yaml.Marshal(s) directly
// would have yaml.v3 ignore those tags and fall back to lowercasing the
// Go field names with no underscores (source_kind would become
// "sourcekind"), which also disagrees with the same result rendered to
// stdout for a run without --artifacts-dir. Marshaling to JSON first and
// decoding that JSON into a generic value before handing it to
// yaml.Marshal is the same technique the get-stats command already uses
// for its stdout rendering (marshalJSONOrYAML in internal/clitree) and
// the one artifact/codec.go's marshalYAML uses for the four cached
// artifact kinds, so that json and yaml renderings of the same value
// never drift apart on field names.
func marshalStats(s stats.Stats, format artifact.Format) (data []byte, ext string, err error) {
	jsonData, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal json: %w", err)
	}

	switch format {
	case artifact.FormatJSON:
		return append(jsonData, '\n'), "json", nil
	case artifact.FormatYAML:
		var generic any
		if err := json.Unmarshal(jsonData, &generic); err != nil {
			return nil, "", fmt.Errorf("decode json for yaml conversion: %w", err)
		}
		out, err := yaml.Marshal(generic)
		if err != nil {
			return nil, "", fmt.Errorf("marshal yaml: %w", err)
		}
		return out, "yaml", nil
	default:
		return nil, "", fmt.Errorf("format %q: %w", format, artifact.ErrUnsupportedFormat)
	}
}
