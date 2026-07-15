// Package contracts builds contracts/openapi.yaml (TZ.md section 10): a
// deterministic OpenAPI 3.1 document with an empty paths object, whose
// components/schemas are generated at build time from Go types via
// jsonschema.ForType.
//
// # Two call sites, deliberately different scopes
//
// The eleven MCP tool input/output schemas (six input, five output) are
// generated with toolForOptions -- the exact same bare
// &jsonschema.ForOptions{} value apps/cmd/bogoslav-mcp's mcp.AddTool
// calls use to infer a tool's inputSchema/outputSchema by reflection
// (setSchema -> jsonschema.ForType(rt, &jsonschema.ForOptions{}), the
// library's own code path -- see apps/internal/mcptool/schema_test.go,
// which proves this for the six tool input types this package also
// reads). Same function, same types, same options value: "one
// generation, two consumers" is a fact of construction, not a
// coincidence checked at runtime.
//
// The four artifact document schemas are generated with
// artifactForOptions instead: the same bare defaults, plus one
// TypeSchemas entry that renders domain.Date as {type: string, format:
// date} rather than the empty `type: object` jsonschema.ForType would
// otherwise infer for it (domain.Date's only exported surface is a
// MarshalJSON method; every field is unexported, so field reflection
// sees nothing -- see domain/date.go and generate.go's dateSchema).
//
// These two options values are allowed to differ precisely because
// their scopes never overlap: domain.Date does not appear, directly or
// through any embedded type, in any of the six tool input types or five
// tool output types this package generates (verified by reading every
// field of every one of apps/internal/mcptool's *Input and *Output
// types: every from/to-shaped field there is a plain wire string,
// parsed into a domain.Date only inside apps/cmd/bogoslav-mcp's
// request-mapping functions -- newFindMRsRequest and friends -- which
// run after mcp.AddTool has already inferred its schema from the raw
// string-typed In/Out struct). Since domain.Date is unreachable from
// any type mcp.AddTool ever reflects on, applying TypeSchemas to the
// four artifact schemas changes nothing about what any tool's
// inputSchema or outputSchema is: there is no AddTool-inferred value
// left for it to diverge from. Using artifactForOptions for a tool
// schema, or toolForOptions for an artifact schema, would be a bug;
// schemaEntries' newSchemaEntry/newArtifactSchemaEntry split exists so
// that mistake cannot happen by a single stray call-site edit.
//
// What is included:
//   - the six MCP tool input types from apps/internal/mcptool
//     (FindMRsInput, GetCommentsInput, GetClassifyBatchInput,
//     SaveLabelsInput, FilterCommentsInput, GetStatsInput);
//   - five of the six MCP tool output types, also from
//     apps/internal/mcptool (FindMRsOutput, GetCommentsOutput,
//     SaveLabelsOutput, FilterCommentsOutput, GetStatsOutput);
//   - the four artifact document types from apps/internal/artifact
//     (MRList, CommentList, LabeledComments, FilteredComments), whose
//     kinds are mr_list, comment_list, labeled_comments and
//     filtered_comments (TZ.md section 4).
//
// What is honestly left out, and why (documented in the generated
// document's info.description, not just here):
//
//	get_classify_batch's output type, mcptool.GetClassifyBatchOutput,
//	embeds *jsonschema.Schema, whose own Go type is self-referential
//	(Defs/Properties/Items point back to Schema). jsonschema.ForType
//	errors on that cycle -- this is the same reason
//	apps/cmd/bogoslav-mcp/tool_get_classify_batch.go registers that
//	tool with mcp.AddTool's Out type parameter as "any" rather than
//	GetClassifyBatchOutput itself. checkGetClassifyBatchOutputCycle
//	asserts this failure keeps happening for the expected reason
//	(a cycle, not something else) every time Generate runs, and
//	Generate refuses to proceed if that assumption stops holding --
//	the honest response to "the cycle got fixed upstream" is to
//	revisit this decision, not to silently keep omitting the schema.
//	A hand-written schema for this one shape was considered and
//	rejected: TZ.md section 10 forbids a manual schema duplicate, and
//	carving out a single hand-written exception inside an otherwise
//	fully generated document would quietly reintroduce exactly the
//	drift this generator exists to prevent.
//
// The other five tools' output types used to live in package main of
// apps/cmd/bogoslav-mcp, alongside the mcp.AddTool calls that use them --
// Go does not allow importing a main package from anywhere, by any other
// package, so no generator living outside that package could call
// jsonschema.ForType on them. They now live in apps/internal/mcptool,
// next to their matching input types and GetClassifyBatchOutput, for
// exactly that reason; apps/cmd/bogoslav-mcp imports them from there, and
// this package can too.
//
// Regenerate with `make -C apps contracts`, or `go generate` from this
// package's directory.
package contracts

//go:generate go run ../../cmd/gen-contracts -out ../../../contracts/openapi.yaml
