package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// TestMain_exitsNonZeroOnError builds the real bogoslav-cli binary and
// runs it with a command that is guaranteed to fail (a required flag
// missing), proving main maps any command error to a non-zero process
// exit code, not just a non-nil in-process error.
func TestMain_exitsNonZeroOnError(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess build in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "bogoslav-cli")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build error = %v, output:\n%s", err, out)
	}

	run := exec.Command(bin, "find-mrs")
	out, err := run.CombinedOutput()
	if err == nil {
		t.Fatalf("running find-mrs without required flags succeeded, want a non-zero exit; output:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("error = %v (%T), want *exec.ExitError", err, err)
	}
	if exitErr.ExitCode() == 0 {
		t.Errorf("ExitCode() = 0, want non-zero")
	}
}

func TestMain_exitsZeroOnSuccessfulHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess build in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "bogoslav-cli")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build error = %v, output:\n%s", err, out)
	}

	run := exec.Command(bin, "--help")
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("running --help error = %v, output:\n%s", err, out)
	}
}
