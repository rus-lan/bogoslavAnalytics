package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackage_neverWritesToStdoutDirectly is the acceptance check for
// TZ.md section 7.1's single easiest way to break an stdio MCP server:
// stdout carries the protocol stream exclusively (mcp.StdioTransport
// owns it), so nothing in this binary's own source may reference
// os.Stdout or call fmt.Print/fmt.Println/fmt.Printf directly. It parses
// every non-test .go file in this directory, plus apps/internal/mcptool
// (the tool input/output types this binary imports and links in), and
// fails on the first such reference, so a stray fmt.Println added later
// breaks this test instead of silently corrupting the protocol stream.
func TestPackage_neverWritesToStdoutDirectly(t *testing.T) {
	var files []string
	for _, pattern := range []string{"*.go", "../../internal/mcptool/*.go"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("filepath.Glob(%q) error = %v", pattern, err)
		}
		files = append(files, matches...)
	}

	forbiddenCalls := map[string]bool{
		"Print":   true,
		"Println": true,
		"Printf":  true,
	}

	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

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
