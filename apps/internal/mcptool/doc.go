// Package mcptool holds bogoslav-mcp's six tool input types (find_mrs,
// get_comments, get_classify_batch, save_labels, filter_comments,
// get_stats), five of the six tools' output types (FindMRsOutput,
// GetCommentsOutput, SaveLabelsOutput, FilterCommentsOutput,
// GetStatsOutput) and GetClassifyBatchOutput, so a build-time
// contracts/openapi.yaml generator (TZ.md section 10) can import the
// exact same Go types apps/cmd/bogoslav-mcp/server.go's mcp.AddTool
// calls infer a tool's JSON Schema from, and call
// jsonschema.For[T](&jsonschema.ForOptions{}) on them itself. Since
// jsonschema.For/ForType is a pure function of the Go type, that makes
// "one generation, two consumers" (the MCP tool schema and
// contracts/openapi.yaml) a fact, not a coincidence: both derive from the
// one type this package declares. The output types live here, next to
// their matching input types, for exactly the same reason the input
// types do: apps/cmd/bogoslav-mcp is package main, and Go forbids
// importing a main package from anywhere, by any other package, so a
// generator outside that package could not otherwise reflect on them.
//
// These types carry no logic of their own -- no GitLab call, no app
// request conversion, no rendering -- only field definitions and their
// jsonschema tags. Converting an input type into its matching
// apps/internal/app request, building an output value from an
// apps/internal/app result, and every other tool handler, stays in
// apps/cmd/bogoslav-mcp: this package is data only.
//
// GetClassifyBatchOutput is the one output type that stays out of
// contracts/openapi.yaml: it embeds *jsonschema.Schema, whose own Go type
// is self-referential (Defs, Properties, Items, ... all point back to
// Schema), so jsonschema.For cannot infer a schema for a type containing
// it at all -- not even from an importable package like this one.
// apps/cmd/bogoslav-mcp/server.go's mcp.AddTool call works around this
// today by registering the get_classify_batch tool with an "any" output
// type parameter (see apps/cmd/bogoslav-mcp/tool_get_classify_batch.go);
// apps/internal/contracts does the same, and documents why (see its own
// doc.go). This is a type-level reflection limit, not a defect:
// marshaling an actual *jsonschema.Schema value to JSON, at call time, is
// unaffected.
package mcptool
