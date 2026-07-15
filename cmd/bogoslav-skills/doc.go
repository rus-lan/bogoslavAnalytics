// Command bogoslav-skills does two jobs (TZ.md section 9): it generates
// the bogoslav Agent Skill (SKILL.md, per the agentskills.io standard) by
// walking internal/clitree's live *cobra.Command tree -- the same
// tree bogoslav-cli itself runs -- so command names, flags, and help text
// can never drift from the CLI; and it installs that skill, plus the
// bogoslav-mcp MCP server registration, into a target agent tool in one
// command.
//
// Five live install targets share one project-relative skill placement
// (.claude/skills/bogoslav/ and .agents/skills/bogoslav/, written
// identically for all five) and one of two MCP config shapes: family A
// (mcpServers, for claude, cursor, cline) and family B (mcp, command as
// one array, environment instead of env, for opencode and kilo). A sixth
// target, aider, has no MCP support at all and gets a degraded
// CONVENTIONS.md instead (TZ.md section 9.3.5). Roo Code is not a target:
// it was archived read-only by its owner on 2026-05-15.
//
// kilo.jsonc (and any other config file here) is merged, never
// overwritten, using github.com/tailscale/hujson: a JWCC (JSON-with-
// comments-and-commas) parser whose Patch method performs a surgical
// RFC 6902 edit, leaving every byte this command does not own --
// comments, formatting, other MCP servers -- untouched. See mcpconfig.go.
package main
