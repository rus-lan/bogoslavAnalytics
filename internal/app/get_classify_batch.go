package app

import (
	"fmt"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/internal/classify"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// labelCacheTTL stands in for "never expires" when GetClassifyBatch
// reuses cache.Lookup to check for an existing artifact-3: the labeling
// cache (TZ.md section 8.4) is keyed by content -- a hash of the batch's
// items plus model and taxonomy version -- never by age, so any existing
// match is a hit regardless of how old it is. cache.Lookup has no
// dedicated "no expiry" value, so this uses a duration far past any
// realistic artifact age instead of teaching Lookup a second freshness
// rule.
const labelCacheTTL = 100 * 365 * 24 * time.Hour

// GetClassifyBatchRequest is the input to GetClassifyBatch, mirroring
// the get_classify_batch tool / classify prepare command surface (TZ.md
// section 7.2).
type GetClassifyBatchRequest struct {
	// CommentListPath is the artifact-2 to read the batch from.
	CommentListPath string
	Model           string
	// Taxonomy is the taxonomy to label against; nil means
	// classify.DefaultTaxonomy() (TZ.md section 8.5).
	Taxonomy *classify.Taxonomy
	// Dir is where GetClassifyBatch looks for an existing artifact-3
	// built from the same batch, model and taxonomy version (TZ.md
	// section 8.4). "" defaults to "artifacts".
	Dir string
}

// GetClassifyBatchResult is the output of GetClassifyBatch: either a
// cache hit naming an existing artifact-3, or a fresh batch plus the
// taxonomy, result JSON schema and rendered prompt the calling agent
// needs to label it (TZ.md section 8.1: "MCP владеет контрактом:
// get_classify_batch отдаёт: батч комментариев, таксономию, JSON-схему
// результата, шаблон промпта"). Without the prompt the agent has data
// and a schema but no instructions -- GetClassifyBatch never calls an
// LLM itself, but it must still hand over everything the calling agent
// needs to run one.
type GetClassifyBatchResult struct {
	Cached bool
	// ArtifactPath is set when Cached is true: the existing
	// labeled_comments file.
	ArtifactPath string

	// Batch, Taxonomy, Schema and Prompt are set when Cached is false.
	Batch    []domain.Note
	Taxonomy classify.Taxonomy
	Schema   *jsonschema.Schema
	// Prompt is classify.RenderPrompt applied to Taxonomy and Batch: the
	// full instructions the calling agent sends to whatever model it
	// runs (TZ.md section 8.1). classify/ owns the prompt text as data;
	// GetClassifyBatch only renders and hands it over.
	Prompt string
}

// GetClassifyBatch is the shared implementation behind the
// get_classify_batch MCP tool and the classify prepare CLI command
// (TZ.md section 7.2): read artifact-2, and either report that an
// unchanged batch already has a matching artifact-3 (TZ.md section 8.4),
// or hand back the batch, taxonomy, result JSON schema and rendered
// prompt for the calling agent to label (TZ.md section 8.1).
// GetClassifyBatch never calls an LLM.
func GetClassifyBatch(req GetClassifyBatchRequest) (GetClassifyBatchResult, error) {
	doc, err := artifact.ReadCommentList(req.CommentListPath)
	if err != nil {
		return GetClassifyBatchResult{}, fmt.Errorf("get classify batch: read %q: %w", req.CommentListPath, err)
	}

	notes := notesOf(doc.Items)
	taxonomy := resolveTaxonomy(req.Taxonomy)

	hash, err := labelArtifactHash(notes, req.Model, taxonomy.Version)
	if err != nil {
		return GetClassifyBatchResult{}, fmt.Errorf("get classify batch: %w", err)
	}

	dir := outDir(req.Dir)
	path, hit, err := cache.Lookup(
		string(artifact.KindLabeledComments), hash,
		cache.Options{Dir: dir, TTL: labelCacheTTL},
		&artifact.HeaderStore{}, time.Now(),
	)
	if err != nil {
		return GetClassifyBatchResult{}, fmt.Errorf("get classify batch: %w", err)
	}
	if hit {
		return GetClassifyBatchResult{Cached: true, ArtifactPath: path, Taxonomy: taxonomy}, nil
	}

	schema, err := classify.ResultSchema()
	if err != nil {
		return GetClassifyBatchResult{}, fmt.Errorf("get classify batch: %w", err)
	}

	prompt, err := classify.RenderPrompt(classify.PromptData{Taxonomy: taxonomy, Notes: notes})
	if err != nil {
		return GetClassifyBatchResult{}, fmt.Errorf("get classify batch: %w", err)
	}

	return GetClassifyBatchResult{
		Batch:    notes,
		Taxonomy: taxonomy,
		Schema:   schema,
		Prompt:   prompt,
	}, nil
}
