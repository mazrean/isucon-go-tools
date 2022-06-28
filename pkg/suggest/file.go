package suggest

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

func FindMainFile(pass *analysis.Pass) (*ast.File, bool) {
	for _, f := range pass.Files {
		if f.Name.Name != "main" {
			continue
		}

		for _, decl := range f.Decls {
			decl, ok := decl.(*ast.FuncDecl)
			if ok && decl.Name.Name == "main" {
				return f, true
			}
		}
	}

	return nil, false
}
