package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// buildFlags registers a command's flags via register on a throwaway
// *cobra.Command, parses args into a fresh flags value, and returns both,
// failing the test immediately on a parse error. It lets a command's
// tests build a flags value the exact way cobra would from real
// command-line args, without going through Execute or RunE -- proving
// the flag -> request-struct mapping without executing the use case.
func buildFlags[F any](t *testing.T, register func(cmd *cobra.Command, flags *F), args []string) (*cobra.Command, *F) {
	t.Helper()
	flags := new(F)
	cmd := &cobra.Command{Use: "test"}
	register(cmd, flags)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatalf("ParseFlags(%v) error = %v", args, err)
	}
	return cmd, flags
}

// writeFileT writes content to path, failing the test immediately on
// error.
func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
