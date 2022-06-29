package importer

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"strconv"
	"strings"

	"github.com/gostaticanalysis/analysisutil"
	"github.com/mazrean/isucon-go-tools/analysers/cache"
	"github.com/mazrean/isucon-go-tools/analysers/db"
	"github.com/mazrean/isucon-go-tools/analysers/echo"
	"github.com/mazrean/isucon-go-tools/analysers/embed"
	"github.com/mazrean/isucon-go-tools/analysers/gin"
	"github.com/mazrean/isucon-go-tools/analysers/http"
	"github.com/mazrean/isucon-go-tools/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

var (
	importAnalyzers = []*analysis.Analyzer{
		embed.Analyzer,
		echo.Analyzer,
		gin.Analyzer,
		http.Analyzer,
		db.Analyzer,
		cache.Analyzer,
	}
)

var Analyzer = &analysis.Analyzer{
	Name:     "importer",
	Doc:      "automatically import packages",
	Run:      run,
	Requires: importAnalyzers,
}

func run(pass *analysis.Pass) (any, error) {
	requiredPkgMap := map[*ast.File][]*suggest.ImportInfo{}
	for _, analyzer := range importAnalyzers {
		pkgs := pass.ResultOf[analyzer].([]*suggest.ImportInfo)

		for _, pkg := range pkgs {
			requiredPkgMap[pkg.File] = append(requiredPkgMap[pkg.File], pkg)
		}
	}

	for f, requiredPkgs := range requiredPkgMap {
		missingPkgs := []*suggest.ImportInfo{}
		pkgMap := makePackageMap(f)
		dupDetecter := map[string]struct{}{}
		for _, pkg := range requiredPkgs {
			if _, ok := dupDetecter[pkg.Path]; ok {
				continue
			}
			dupDetecter[pkg.Path] = struct{}{}

			if _, ok := pkgMap[analysisutil.RemoveVendor(pkg.Path)]; !ok {
				missingPkgs = append(missingPkgs, pkg)
			}
		}

		if len(missingPkgs) == 0 {
			return nil, nil
		}

		importDecl, ok := findImportDecl(f)
		if !ok {
			// TODO: add import statement
			return nil, nil
		}

		newImportDecl := fixedImportDecl(importDecl, missingPkgs)

		err := suggestChanges(pass, missingPkgs, importDecl, newImportDecl)
		if err != nil {
			return nil, fmt.Errorf("failed to suggest changes: %w", err)
		}
	}

	return nil, nil
}

func makePackageMap(f *ast.File) map[string]struct{} {
	pkgMap := make(map[string]struct{}, len(f.Imports))
	for _, pkg := range f.Imports {
		path, err := strconv.Unquote(pkg.Path.Value)
		if err != nil {
			continue
		}

		pkgMap[analysisutil.RemoveVendor(path)] = struct{}{}
	}

	return pkgMap
}

func findImportDecl(f *ast.File) (*ast.GenDecl, bool) {
	for _, decl := range f.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if ok && decl.Tok == token.IMPORT {
			return decl, true
		}
	}

	return nil, false
}

func fixedImportDecl(decl *ast.GenDecl, missingPkgs []*suggest.ImportInfo) *ast.GenDecl {
	newImportSpecs := []ast.Spec{}
	for _, pkg := range missingPkgs {
		var ident *ast.Ident = nil
		if len(pkg.Ident) != 0 {
			ident = ast.NewIdent(pkg.Ident)
		}

		newImportSpecs = append(newImportSpecs, &ast.ImportSpec{
			Name: ident,
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(pkg.Path),
			},
		})
	}

	newDecl := &ast.GenDecl{
		Doc:   decl.Doc,
		Tok:   decl.Tok,
		Specs: append(decl.Specs, newImportSpecs...),
	}

	return newDecl
}

func suggestChanges(pass *analysis.Pass, missingPkgInfos []*suggest.ImportInfo, decl *ast.GenDecl, newDecl *ast.GenDecl) error {
	buf := bytes.Buffer{}

	err := format.Node(&buf, pass.Fset, newDecl)
	if err != nil {
		return fmt.Errorf("failed to format import declaration: %w", err)
	}

	missingPkgs := []string{}
	for _, pkg := range missingPkgInfos {
		missingPkgs = append(missingPkgs, pkg.Path)
	}
	missingPkgsStr := strings.Join(missingPkgs, ", ")

	pass.Report(analysis.Diagnostic{
		Pos:     decl.Pos(),
		Message: fmt.Sprintf("missing %s package", missingPkgsStr),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fmt.Sprintf("import %s package", missingPkgsStr),
			TextEdits: []analysis.TextEdit{{
				Pos:     decl.Pos(),
				End:     decl.End(),
				NewText: buf.Bytes(),
			}},
		}},
	})

	return nil
}
