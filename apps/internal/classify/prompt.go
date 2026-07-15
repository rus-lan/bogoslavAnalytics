package classify

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// PromptTemplateText is the prompt template text this package owns. A
// future get_classify_batch MCP tool hands the rendered text to the
// calling agent so the agent can label a batch of comments (TZ.md
// section 8.1); classify itself never sends it anywhere.
const PromptTemplateText = `You are labeling GitLab merge request review comments by their logical, semantic meaning.

Taxonomy version {{.Taxonomy.Version}}. Assign exactly one label to each comment below, chosen only from this set:
{{range .Taxonomy.Labels}}- {{.}}
{{end}}
Use "other" only when none of the other labels fit.

Reply with a single JSON array and nothing else, in this exact shape:
[{"note_id": <integer>, "label": "<one label from the set above>"}, ...]

The array must have exactly one entry for every note_id listed below: no note_id left out, no note_id repeated, no note_id added that is not listed.

Comments:
{{range .Notes}}- note_id {{.ID}}: {{.Body}}
{{end}}`

// promptTemplate is PromptTemplateText, parsed once at package load.
var promptTemplate = template.Must(template.New("classify_prompt").Parse(PromptTemplateText))

// PromptData is what RenderPrompt fills PromptTemplateText with: the
// taxonomy the agent must label against, and the batch of notes to
// label.
type PromptData struct {
	Taxonomy Taxonomy
	Notes    []domain.Note
}

// RenderPrompt fills PromptTemplateText with data and returns the
// finished prompt text (TZ.md section 8.1). classify only builds this
// text; sending it to a model is the calling agent's job, never this
// package's.
func RenderPrompt(data PromptData) (string, error) {
	var b strings.Builder
	if err := promptTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("render prompt: %w", err)
	}
	return b.String(), nil
}
