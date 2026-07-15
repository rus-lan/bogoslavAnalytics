package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// serverName is the one MCP server this command ever installs (TZ.md
// section 9.1: "один скилл bogoslav"). It is also the skill's directory
// name and its frontmatter name.
const serverName = "bogoslav"

// serverDescriptor is the canonical MCP server descriptor (TZ.md section
// 9.2): name, command, args, env. Transport is not a field because this
// command only ever ships stdio (TZ.md section 7.1): every live target
// spawns bogoslav-mcp as a local child process, so there is no transport
// value left to choose.
type serverDescriptor struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// newDescriptor builds the descriptor for bogoslav-mcp at mcpCommand.
// Args and Env are never nil so the two config families (mcpconfig.go)
// can always marshal them as JSON "[]" and "{}", never "null".
func newDescriptor(mcpCommand string) serverDescriptor {
	return serverDescriptor{
		Name:    serverName,
		Command: mcpCommand,
		Args:    []string{},
		Env:     map[string]string{},
	}
}

// resolveMCPCommand finds the bogoslav-mcp binary this command's config
// entries should point at. An explicit --mcp-command always wins. Failing
// that, it looks for a binary named bogoslav-mcp next to bogoslav-skills'
// own executable -- the layout every build of this module produces
// (apps/bin/bogoslav-cli, apps/bin/bogoslav-mcp, apps/bin/bogoslav-skills
// side by side). If neither is available, it falls back to the bare name
// "bogoslav-mcp" and reports on stderr that it is trusting PATH, since a
// config that points at nothing found on disk is a config that silently
// fails to start.
func resolveMCPCommand(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}

	self, err := os.Executable()
	if err != nil {
		return mcpBinaryName(), nil
	}
	return siblingOrPathFallback(self), nil
}

// siblingOrPathFallback is resolveMCPCommand's testable core: given
// self (bogoslav-skills' own executable path), it returns the sibling
// bogoslav-mcp binary if one exists on disk next to it, or the bare
// binary name otherwise. Splitting this out of resolveMCPCommand lets
// tests control self directly instead of needing to mock os.Executable.
func siblingOrPathFallback(self string) string {
	sibling := filepath.Join(filepath.Dir(self), mcpBinaryName())
	if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
		return sibling
	}
	return mcpBinaryName()
}

// mcpBinaryName is bogoslav-mcp's expected file name for the current
// platform.
func mcpBinaryName() string {
	if runtime.GOOS == "windows" {
		return "bogoslav-mcp.exe"
	}
	return "bogoslav-mcp"
}

// reportIfPathFallback tells the caller on stderr when resolveMCPCommand
// could not find a sibling binary and fell back to a bare name resolved
// through PATH at run time, since that is silent otherwise.
func reportIfPathFallback(w io.Writer, mcpCommand string) {
	if mcpCommand == "bogoslav-mcp" || mcpCommand == "bogoslav-mcp.exe" {
		fmt.Fprintf(w, "warning: no bogoslav-mcp binary found next to bogoslav-skills; "+
			"writing %q as-is and trusting PATH at run time (pass --mcp-command to pin an exact path)\n", mcpCommand)
	}
}
