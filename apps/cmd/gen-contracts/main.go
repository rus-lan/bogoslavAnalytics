// Command gen-contracts writes contracts/openapi.yaml from
// apps/internal/contracts's deterministic generator (TZ.md section 10).
// All schema-building logic lives in apps/internal/contracts; this
// command is the thin entry point that calls it and writes the result,
// the same cmd/-holds-no-logic convention apps/cmd/bogoslav-cli and
// apps/cmd/bogoslav-mcp already follow for their own internal packages.
//
// Run via `make -C apps contracts`, or `go generate` from
// apps/internal/contracts (see that package's go:generate directive).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/contracts"
)

func main() {
	out := flag.String("out", "../contracts/openapi.yaml",
		"path to write the generated OpenAPI document to; the default assumes "+
			"the current working directory is apps/, matching `make -C apps contracts`")
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
