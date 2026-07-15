// Package mcptool holds bogoslav-mcp's six tool input types (find_mrs,
// get_comments, get_classify_batch, save_labels, filter_comments,
// get_stats) and GetClassifyBatchOutput, so a build-time
// contracts/openapi.yaml generator (TZ.md section 10, a later wave) can
// import the exact same Go types apps/cmd/bogoslav-mcp/server.go's
// mcp.AddTool calls infer a tool's JSON Schema from, and call
// jsonschema.For[T](&jsonschema.ForOptions{}) on them itself. Since
// jsonschema.For/ForType is a pure function of the Go type, that makes
// "one generation, two consumers" (the MCP tool schema and
// contracts/openapi.yaml) a fact, not a coincidence: both derive from the
// one type this package declares.
//
// These types carry no logic of their own -- no GitLab call, no app
// request conversion, no rendering -- only field definitions and their
// jsonschema tags. Converting an input type into its matching
// apps/internal/app request, and every other tool handler, stays in
// apps/cmd/bogoslav-mcp: this package is data only.
//
// GetClassifyBatchOutput is a second, independent wrinkle for that same
// future generator: it embeds *jsonschema.Schema, whose own Go type is
// self-referential (Defs, Properties, Items, ... all point back to
// Schema), so jsonschema.For cannot infer a schema for a type containing
// it at all -- not even from an importable package like this one.
// apps/cmd/bogoslav-mcp/server.go's mcp.AddTool call works around this
// today by registering the get_classify_batch tool with an "any" output
// type parameter (see apps/cmd/bogoslav-mcp/tool_get_classify_batch.go);
// a future generator will need the same workaround, or a hand-written
// schema for that one output shape. This is a type-level reflection
// limit, not a defect: marshaling an actual *jsonschema.Schema value to
// JSON, at call time, is unaffected.
package mcptool
