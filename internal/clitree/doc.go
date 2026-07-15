// Package clitree builds the bogoslav-cli command tree: every command
// mirrors one bogoslav-mcp tool and calls exactly one internal/app
// function, so the CLI and the MCP server can never drift into two
// implementations of the same use case (TZ.md section 7.3).
//
// cmd/bogoslav-cli's main package only calls NewRootCmd and executes
// it; every command's construction, flag registration, and rendering
// lives here instead, because bogoslav-skills (a later wave) generates
// SKILL.md by walking this tree, and Go does not allow importing a
// package main.
package clitree
