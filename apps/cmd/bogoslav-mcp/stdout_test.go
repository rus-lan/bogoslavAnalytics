package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackage_neverWritesToStdoutDirectly is the acceptance check for
// TZ.md section 7.1's single easiest way to break an stdio MCP server:
// stdout carries the protocol stream exclusively (mcp.StdioTransport
// owns it), so nothing in this binary's own source, nor in any
// first-party package this binary links in (transitively, via
// "go list -deps"), may reference os.Stdout or call
// fmt.Print/fmt.Println/fmt.Printf directly. Driving the file list from
// the real dependency graph (instead of a couple of hardcoded globs)
// means a newly added internal package is covered automatically, and a
// stray fmt.Println added anywhere in that graph breaks this test
// instead of silently corrupting the protocol stream.
//
// fmt.Fprintf/Fprintln/Fprint calls are not flagged by name, because
// they write to whatever io.Writer the caller passes in (os.Stderr, a
// bytes.Buffer, cmd.OutOrStdout(), ...) and are the correct way to
// produce output outside of this binary. A call that passes os.Stdout
// as that writer is still caught, because the os.Stdout selector itself
// is forbidden everywhere in the scanned files, regardless of which
// call it appears in.
func TestPackage_neverWritesToStdoutDirectly(t *testing.T) {
	files := firstPartyGoFiles(t)
	if len(files) == 0 {
		t.Fatal("firstPartyGoFiles returned no files; dependency discovery is broken")
	}

	forbiddenCalls := map[string]bool{
		"Print":   true,
		"Println": true,
		"Printf":  true,
	}

	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}

			switch pkgIdent.Name {
			case "os":
				if sel.Sel.Name == "Stdout" {
					t.Errorf("%s: references os.Stdout directly; stdout is the MCP transport", fset.Position(n.Pos()))
				}
			case "fmt":
				if forbiddenCalls[sel.Sel.Name] {
					t.Errorf("%s: calls fmt.%s, which writes to stdout by default; use slog (stderr) instead", fset.Position(n.Pos()), sel.Sel.Name)
				}
			}
			return true
		})
	}
}

// depPackage is the subset of "go list -json" output this test needs.
type depPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
}

// firstPartyGoFiles returns every non-test .go file in this package and
// every package it imports, transitively, that belongs to this module
// (as opposed to the standard library or a third-party module, neither
// of which this binary's tests can police). The package list comes from
// "go list -deps -json .", not a hardcoded set of directories, so it
// stays correct as the import graph grows.
func firstPartyGoFiles(t *testing.T) []string {
	t.Helper()

	modOut, err := exec.Command("go", "list", "-m").Output()
	if err != nil {
		t.Fatalf("go list -m error = %v", err)
	}
	module := strings.TrimSpace(string(modOut))

	depsOut, err := exec.Command("go", "list", "-deps", "-json", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps -json . error = %v", err)
	}

	var files []string
	dec := json.NewDecoder(bytes.NewReader(depsOut))
	for dec.More() {
		var pkg depPackage
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decoding go list -json output: %v", err)
		}
		if pkg.Standard {
			continue // stdlib: not policed here
		}
		if pkg.ImportPath != module && !strings.HasPrefix(pkg.ImportPath, module+"/") {
			continue // third-party module: not policed here
		}

		matches, err := filepath.Glob(filepath.Join(pkg.Dir, "*.go"))
		if err != nil {
			t.Fatalf("filepath.Glob(%q) error = %v", pkg.Dir, err)
		}
		for _, path := range matches {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			files = append(files, path)
		}
	}
	return files
}
