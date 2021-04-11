package imporder

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"
)

type importType int

const (
	internal importType = iota
	external
	standard
)

type impLinter struct {
	pass       *analysis.Pass
	baseImport string
}

func (il *impLinter) getType(in string) importType {
	if strings.HasPrefix(in, il.baseImport) {
		return internal
	}

	parts := strings.Split(in, "/")

	if strings.Contains(parts[0], ".") {
		return external
	}

	return standard
}

func (il *impLinter) sortImports(imps []string) []string {
	var (
		project []string
		global  []string
		std     []string
	)

	for _, imp := range imps {
		if imp == "" {
			continue
		}

		switch il.getType(imp) {
		case internal:
			project = append(project, imp)
		case external:
			global = append(global, imp)
		case standard:
			std = append(std, imp)
		}
	}
	total := len(std) + len(project) + len(global)
	if total == 0 {
		return []string{}
	}

	result := make([]string, 0, total+3)
	if len(std) > 0 {
		sort.Strings(std)
		result = append(result, std...)
		result = append(result, "")
	}

	if len(global) > 0 {
		sort.Strings(global)
		result = append(result, global...)
		result = append(result, "")
	}

	if len(project) > 0 {
		sort.Strings(project)
		result = append(result, project...)
		result = append(result, "")
	}

	return result[:len(result)-1]
}

func (il *impLinter) extractImport(spec ast.Spec) string {
	imp := spec.(*ast.ImportSpec)
	return strings.Trim(imp.Path.Value, `"`)
}

func (il *impLinter) checkImportOrder(block *ast.GenDecl) {
	if !block.Lparen.IsValid() {
		return
	}

	firstPos := il.pass.Fset.Position(block.Lparen)
	lastPos := il.pass.Fset.Position(block.Rparen)
	count := len(block.Specs)

	size := lastPos.Line - firstPos.Line
	if size-count < 1 {
		il.pass.Report(analysis.Diagnostic{
			Pos:      block.Pos(),
			End:      block.End(),
			Category: "import",
			Message:  "incorrect paren position",
		})

		return
	}

	imports := make([]string, size-1)
	for _, spec := range block.Specs {
		ln := il.pass.Fset.Position(spec.Pos())
		idx := ln.Line - firstPos.Line - 1
		imports[idx] = il.extractImport(spec)
	}

	sorted := il.sortImports(imports)

	if len(sorted) != len(imports) {
		il.pass.Report(analysis.Diagnostic{
			Pos:      block.Pos(),
			End:      block.End(),
			Category: "import",
			Message:  fmt.Sprintf("should be \n%s", strings.Join(sorted, "\n")),
		})
		return
	}

	for i := range sorted {
		if sorted[i] != imports[i] {
			il.pass.Report(analysis.Diagnostic{
				Pos:      block.Pos(),
				End:      block.End(),
				Category: "import",
				Message:  fmt.Sprintf("should be \n%s", strings.Join(sorted, "\n")),
			})

			return
		}
	}
}

func (il *impLinter) findImportBlock(fl *ast.File) {
	var blocks []*ast.GenDecl
	ast.Inspect(fl, func(n ast.Node) bool {
		if t, ok := n.(*ast.GenDecl); ok {
			if t.Tok != token.IMPORT {
				return true
			}
			blocks = append(blocks, t)
		}

		return true
	})

	if len(blocks) == 0 {
		return
	}

	if len(blocks) > 1 {
		for b := 1; b < len(blocks); b++ {
			il.pass.Report(analysis.Diagnostic{
				Pos:      blocks[b].Pos(),
				End:      blocks[b].End(),
				Category: "import",
				Message:  "multiple import",
			})
		}
		return
	}

	il.checkImportOrder(blocks[0])
}

func (il *impLinter) findImports() {
	for _, fl := range il.pass.Files {
		// TODO: ignore generated file
		il.findImportBlock(fl)
	}

	return
}

func (il *impLinter) process() error {
	il.findImports()
	return nil
}

func run(pass *analysis.Pass) (interface{}, error) {
	il := impLinter{
		pass:       pass,
		baseImport: pass.Analyzer.Flags.Lookup("base-import").Value.String(),
	}
	return nil, il.process()
}

func NewImportAnalyzer() *analysis.Analyzer {
	fl := flag.NewFlagSet("import", flag.ExitOnError)
	fl.String("base-import", "", "Base import path")
	return &analysis.Analyzer{
		Name:             "import_order",
		Doc:              "sort imports",
		Flags:            *fl,
		Run:              run,
		RunDespiteErrors: false,
	}
}
