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
// apps/internal/mcptool's six *Input types -- the ones mcp.AddTool infers
// a tool's JSON Schema from (github.com/google/jsonschema-go, draft
// 2020-12; see apps/internal/mcptool/schema_test.go). Those types (and
// GetClassifyBatchOutput) live in mcptool, not here, precisely so a
// future contracts/ generator (its own package, per TZ.md section 10's
// "go generate" command) can import them too and call
// jsonschema.For[T](&jsonschema.ForOptions{}) on the exact same Go types
// this package's AddTool calls use: "one generation, two consumers" is a
// fact of the import graph, not a coincidence.
//
// GetClassifyBatchOutput is a second, independent wrinkle for that same
// generator, documented on its own type in apps/internal/mcptool: it
// embeds *jsonschema.Schema, whose own Go type is self-referential (Defs,
// Properties, Items, ... all point back to Schema), so jsonschema.For
// cannot infer a schema for a type containing it at all -- not even from
// an importable package. AddTool works around this today by registering
// the tool with an "any" Out type parameter (see
// tool_get_classify_batch.go); a future generator will need the same
// workaround, or a hand-written schema for that one output shape.
package main
