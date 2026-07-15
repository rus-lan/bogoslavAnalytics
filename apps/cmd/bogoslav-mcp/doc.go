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
// This package's six *Input/*Output types are the ones mcp.AddTool
// infers a tool's JSON Schema from (github.com/google/jsonschema-go,
// draft 2020-12; see schema_test.go). TZ.md section 10 asks whether the
// exact same Go types can also feed a build-time
// jsonschema.For[T](&jsonschema.ForOptions{}) call for
// contracts/openapi.yaml, so the two never drift.
//
// Mechanically, yes: jsonschema.For/ForType is a pure function of the Go
// type, so calling it again on the same type produces an identical
// schema. But as written today, that second call is not actually
// reachable from anywhere else: these types live in package main
// (apps/cmd/bogoslav-mcp), and Go does not allow importing a main
// package. A future contracts/ generator (its own package, per TZ.md
// section 10's "go generate" command) cannot import FindMRsInput,
// GetCommentsInput, and so on from here.
//
// Closing that gap -- something this task's scope (touch only
// apps/cmd/bogoslav-mcp/) does not permit -- needs the six *Input types
// (and any *Output type that is registered with a concrete, not "any",
// Out type parameter) moved to an importable package, for example a new
// apps/internal/mcptool/, imported by both this package's AddTool calls
// and the contracts/ generator. Until that move happens, "one
// generation, two consumers" (TZ.md section 10) is a design goal this
// package satisfies for its own registration, but not yet a fact
// contracts/openapi.yaml can rely on.
//
// GetClassifyBatchOutput is a second, independent wrinkle for that same
// future generator, documented on its own type: it embeds
// *jsonschema.Schema, whose own Go type is self-referential (Defs,
// Properties, Items, ... all point back to Schema), so jsonschema.For
// cannot infer a schema for a type containing it at all -- not even from
// an importable package. AddTool works around this today by registering
// the tool with an "any" Out type parameter (see tool_get_classify_batch.go);
// a future generator will need the same workaround, or a hand-written
// schema for that one output shape.
package main
