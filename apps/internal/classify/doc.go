// Package classify owns the semantic-labeling contract: the label
// taxonomy (Taxonomy, DefaultTaxonomy, NewTaxonomy), the prompt
// template handed to the calling agent (PromptTemplateText,
// RenderPrompt), the JSON Schema of a labeling result plus its
// validator (NoteLabel, ResultSchema, Validate), and the classifier
// provenance builder (NewClassifier). Labeling itself is done by the
// calling agent (opencode, Claude, or similar) outside this process:
// classify never calls an LLM, opens a network connection, or reads or
// writes an artifact (TZ.md section 8.1, section 2.3).
package classify
