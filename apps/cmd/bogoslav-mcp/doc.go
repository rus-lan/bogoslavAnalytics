// Command bogoslav-mcp exposes apps/internal/app's six use cases as MCP
// tools over stdio (TZ.md sections 7.1-7.3): find_mrs, get_comments,
// get_classify_batch, save_labels, filter_comments, get_stats. Tool
// names are snake_case, distinct from bogoslav-cli's kebab-case commands
// (TZ.md section 7.2), but both bind to the exact same apps/internal/app
// function per operation -- there is no second implementation of any use
// case here.
//
// # contracts/openapi.yaml (TZ.md section 10)
//
// This package's tool handlers take their input as one of
// apps/internal/mcptool's six *Input types, and five of the six return
// one of mcptool's matching *Output types -- the ones mcp.AddTool infers
// a tool's JSON Schema from (github.com/google/jsonschema-go, draft
// 2020-12; see apps/internal/mcptool/schema_test.go). Those types (and
// GetClassifyBatchOutput) live in mcptool, not here, precisely so
// apps/internal/contracts's generator (TZ.md section 10's "go generate"
// command) can import them too and call
// jsonschema.For[T](&jsonschema.ForOptions{}) on the exact same Go types
// this package's AddTool calls use: "one generation, two consumers" is a
// fact of the import graph, not a coincidence.
//
// GetClassifyBatchOutput is the one output type that stays here as an
// "any" Out type parameter (see tool_get_classify_batch.go), rather than
// as itself: it embeds *jsonschema.Schema, whose own Go type is
// self-referential (Defs, Properties, Items, ... all point back to
// Schema), so jsonschema.For cannot infer a schema for a type containing
// it at all -- not even from an importable package like mcptool. This is
// documented in full on GetClassifyBatchOutput itself, in
// apps/internal/mcptool, and in apps/internal/contracts's own doc.go.
package main
