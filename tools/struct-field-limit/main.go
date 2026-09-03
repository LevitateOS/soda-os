package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maximumStructFields = 10

type violation struct {
	path   string
	line   int
	fields int
}

type scanner struct {
	fset       *token.FileSet
	violations []violation
}

type structVisitor struct {
	path       string
	fset       *token.FileSet
	violations *[]violation
}

var ignoredDirectories = map[string]struct{}{
	".artifacts": {},
	".git":       {},
	"vendor":     {},
}

func main() {
	state := scanner{fset: token.NewFileSet()}
	err := filepath.WalkDir(".", state.visitPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sort.Slice(state.violations, func(left, right int) bool {
		first := state.violations[left]
		second := state.violations[right]
		return first.path < second.path || first.path == second.path && first.line < second.line
	})
	for _, item := range state.violations {
		fmt.Printf("%s:%d: struct has %d fields; maximum is %d\n", item.path, item.line, item.fields, maximumStructFields)
	}
	if len(state.violations) > 0 {
		os.Exit(1)
	}
}

func (state *scanner) visitPath(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() {
		if path != "." {
			if _, ignored := ignoredDirectories[entry.Name()]; ignored {
				return filepath.SkipDir
			}
		}
		return nil
	}
	if filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), ".pb.go") {
		return nil
	}
	file, err := parser.ParseFile(state.fset, path, nil, 0)
	if err != nil {
		return err
	}
	visitor := structVisitor{path: path, fset: state.fset, violations: &state.violations}
	ast.Walk(visitor, file)
	return nil
}

func (visitor structVisitor) Visit(node ast.Node) ast.Visitor {
	structure, ok := node.(*ast.StructType)
	if !ok {
		return visitor
	}
	fields := 0
	for _, field := range structure.Fields.List {
		fieldCount := len(field.Names)
		if fieldCount == 0 {
			fieldCount = 1
		}
		fields += fieldCount
	}
	if fields <= maximumStructFields {
		return visitor
	}
	position := visitor.fset.Position(structure.Pos())
	*visitor.violations = append(*visitor.violations, violation{
		path:   strings.TrimPrefix(visitor.path, "./"),
		line:   position.Line,
		fields: fields,
	})
	return visitor
}
