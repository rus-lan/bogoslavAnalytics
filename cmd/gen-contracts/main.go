// Command gen-contracts writes contracts/openapi.yaml from
// internal/contracts's deterministic generator (TZ.md section 10).
// All schema-building logic lives in internal/contracts; this
// command is the thin entry point that calls it and writes the result,
// the same cmd/-holds-no-logic convention cmd/bogoslav-cli and
// cmd/bogoslav-mcp already follow for their own internal packages.
//
// Run via `make contracts`, or `go generate` from
// internal/contracts (see that package's go:generate directive).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rus-lan/bogoslavAnalytics/internal/contracts"
)

func main() {
	out := flag.String("out", "contracts/openapi.yaml",
		"path to write the generated OpenAPI document to; the default assumes "+
			"the current working directory is the repo root, matching `make contracts`")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "gen-contracts:", err)
		os.Exit(1)
	}
}

func run(out string) error {
	data, err := contracts.Generate()
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", out, err)
	}
	return nil
}
