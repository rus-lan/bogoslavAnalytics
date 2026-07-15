package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCmd_helpListsBothSubcommands(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help: %v", err)
	}

	for _, want := range []string{"generate", "install"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--help output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestRootCmd_unknownCommandFails(t *testing.T) {
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"not-a-real-command"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}
