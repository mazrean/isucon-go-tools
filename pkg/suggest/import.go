package suggest

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strconv"

	"github.com/gostaticanalysis/analysisutil"
	"golang.org/x/tools/go/analysis"
)

func ImportPackage(pass *analysis.Pass, f *ast.File, pkgIdent string, pkgName string) error {
	if searchEmbedPackage(f, pkgName) {
		return nil
	}

	lastImportPos, ok := findLastImportPos(f)
	if !ok {
		// TODO: add import statement
		return nil
	}

	err := suggestChanges(pass, pkgIdent, pkgName, lastImportPos)
	if err != nil {
		return fmt.Errorf("failed to suggest changes: %w", err)
	}

	return nil
}

func searchEmbedPackage(f *ast.File, pkgName string) bool {
	for _, pkg := range f.Imports {
		path, err := strconv.Unquote(pkg.Path.Value)
		if err != nil {
			continue
		}

		if analysisutil.RemoveVendor(path) == pkgName {
			return true
		}
	}

	return false
}

func findLastImportPos(f *ast.File) (token.Pos, bool) {
	var importDecl *ast.GenDecl
	for _, decl := range f.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if ok && decl.Tok == token.IMPORT {
			importDecl = decl
			break
		}
	}
	if importDecl == nil {
		return token.NoPos, false
	}

	if len(importDecl.Specs) == 0 {
		return importDecl.Lparen, false
	}

	return importDecl.Specs[len(importDecl.Specs)-1].End(), true
}

func suggestChanges(pass *analysis.Pass, pkgIdent, pkgName string, lastImportPos token.Pos) error {
	buf := bytes.Buffer{}

	var ident *ast.Ident = nil
	if len(pkgIdent) != 0 {
		ident = ast.NewIdent(pkgIdent)
	}

	_, err := buf.WriteString("\n\t")
	if err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	err = format.Node(&buf, pass.Fset, &ast.ImportSpec{
		Name: ident,
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"` + pkgName + `"`,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to format import declaration: %w", err)
	}

	pass.Report(analysis.Diagnostic{
		Pos:     lastImportPos,
		Message: fmt.Sprintf("missing %s package", pkgName),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fmt.Sprintf("import %s package", pkgName),
			TextEdits: []analysis.TextEdit{{
				Pos:     lastImportPos,
				End:     lastImportPos,
				NewText: buf.Bytes(),
			}},
		}},
	})

	return nil
}
