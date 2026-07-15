// Package contracts builds contracts/openapi.yaml (TZ.md section 10): a
// deterministic OpenAPI 3.1 document with an empty paths object, whose
// components/schemas are generated at build time from the exact same Go
// types apps/cmd/bogoslav-mcp's mcp.AddTool calls infer a tool's
// inputSchema from by reflection
// (jsonschema.ForType(rt, &jsonschema.ForOptions{}), the library's own
// code path -- see apps/internal/mcptool/schema_test.go, which proves
// this for the six tool input types this package also reads). Same
// function, same types, called the same way: "one generation, two
// consumers" is a fact of construction, not a coincidence checked at
// runtime.
//
// What is included:
//   - the six MCP tool input types from apps/internal/mcptool
//     (FindMRsInput, GetCommentsInput, GetClassifyBatchInput,
//     SaveLabelsInput, FilterCommentsInput, GetStatsInput);
//   - the four artifact document types from apps/internal/artifact
//     (MRList, CommentList, LabeledComments, FilteredComments), whose
//     kinds are mr_list, comment_list, labeled_comments and
//     filtered_comments (TZ.md section 4).
//
// What is honestly left out, and why (both documented in the generated
// document's info.description, not just here):
//
//  1. get_classify_batch's output type, mcptool.GetClassifyBatchOutput,
//     embeds *jsonschema.Schema, whose own Go type is self-referential
//     (Defs/Properties/Items point back to Schema). jsonschema.ForType
//     errors on that cycle -- this is the same reason
//     apps/cmd/bogoslav-mcp/tool_get_classify_batch.go registers that
//     tool with mcp.AddTool's Out type parameter as "any" rather than
//     GetClassifyBatchOutput itself. checkGetClassifyBatchOutputCycles
//     asserts this failure keeps happening for the expected reason
//     (a cycle, not something else) every time Generate runs, and
//     Generate refuses to proceed if that assumption stops holding --
//     the honest response to "the cycle got fixed upstream" is to
//     revisit this decision, not to silently keep omitting the schema.
//     A hand-written schema for this one shape was considered and
//     rejected: TZ.md section 10 forbids a manual schema duplicate, and
//     carving out a single hand-written exception inside an otherwise
//     fully generated document would quietly reintroduce exactly the
//     drift this generator exists to prevent.
//  2. The other five tools' output types (FindMRsOutput,
//     GetCommentsOutput, SaveLabelsOutput, FilterCommentsOutput,
//     GetStatsOutput) are defined in package main of
//     apps/cmd/bogoslav-mcp, alongside the mcp.AddTool calls that use
//     them. Go does not allow importing a main package from anywhere,
//     by any other package -- this is a language rule, not a missing
//     export or an oversight -- so no generator living outside that
//     package can call jsonschema.ForType on them. Moving them into an
//     importable package (mcptool, alongside the input types and
//     GetClassifyBatchOutput) would need edits to
//     apps/cmd/bogoslav-mcp and apps/internal/mcptool, both out of this
//     change's scope; hand-typing schemas for them here would be the
//     same forbidden manual duplicate as (1). They are left out
//     entirely, rather than faked, and the omission is recorded in the
//     generated document.
//
// Regenerate with `make -C apps contracts`, or `go generate` from this
// package's directory.
package contracts

//go:generate go run ../../cmd/gen-contracts -out ../../../contracts/openapi.yaml
