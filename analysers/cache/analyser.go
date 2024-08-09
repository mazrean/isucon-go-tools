package cache

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"reflect"

	"github.com/mazrean/isucon-go-tools/v2/pkg/analyze"
	"github.com/mazrean/isucon-go-tools/v2/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

const (
	cachePkgName         = "github.com/mazrean/isucon-go-tools/v2/cache"
	cachePkgDefaultIdent = "isucache"
	cachePurgeFuncName   = "AllPurge"
	initializeKeyword    = "initialize"
)

var (
	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "cache",
		Doc:        "automatically setup cache purge",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
	}
)

func run(pass *analysis.Pass) (any, error) {
	var (
		initializeFuncFile *ast.File
		initializeFuncDecl *ast.FuncDecl
		initializeFuncName string
	)
	for _, f := range pass.Files {
		for _, decl := range f.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl == nil {
				continue
			}

			funcName := funcDecl.Name
			if funcName == nil {
				continue
			}

			if analyze.IsInitializeFuncName(funcName.Name) {
				initializeFuncFile = f
				initializeFuncDecl = funcDecl
				initializeFuncName = funcName.Name
				break
			}
		}
	}
	if initializeFuncDecl == nil {
		return importPkgs, nil
	}

	if initializeFuncDecl.Body != nil {
		purgeCalled := false
		ast.Inspect(initializeFuncDecl.Body, func(n ast.Node) bool {
			if purgeCalled {
				return false
			}

			callExpr, ok := n.(*ast.CallExpr)
			if !ok || callExpr == nil {
				return true
			}

			selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
			if !ok || selectorExpr == nil {
				return true
			}

			pkgIdent, ok := selectorExpr.X.(*ast.Ident)
			if !ok || pkgIdent == nil {
				return true
			}

			if pkgIdent.Name == cachePkgDefaultIdent && selectorExpr.Sel.Name == cachePurgeFuncName {
				purgeCalled = true
				return false
			}

			return true
		})

		if purgeCalled {
			return importPkgs, nil
		}
	}

	importPkgs = append(importPkgs, &suggest.ImportInfo{
		File:  initializeFuncFile,
		Ident: cachePkgDefaultIdent,
		Path:  cachePkgName,
	})

	buf := bytes.Buffer{}

	var list []ast.Stmt
	if initializeFuncDecl.Body == nil {
		list = make([]ast.Stmt, 0, 1)
	} else {
		list = make([]ast.Stmt, 0, len(initializeFuncDecl.Body.List)+1)
	}
	list = append(list, &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(cachePkgDefaultIdent),
				Sel: ast.NewIdent(cachePurgeFuncName),
			},
			Args: []ast.Expr{},
		},
	})
	list = append(list, initializeFuncDecl.Body.List...)

	err := format.Node(&buf, pass.Fset, &ast.BlockStmt{
		List: list,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to format import declaration: %w", err)
	}

	var (
		pos token.Pos
		end token.Pos
	)
	if initializeFuncDecl.Body == nil {
		pos = initializeFuncDecl.End()
		end = initializeFuncDecl.End()
	} else {
		pos = initializeFuncDecl.Body.Pos()
		end = initializeFuncDecl.Body.End()
	}

	pass.Report(analysis.Diagnostic{
		Pos:     initializeFuncDecl.Pos(),
		Message: fmt.Sprintf("%s should call (%s).%s", initializeFuncName, cachePkgName, cachePurgeFuncName),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fmt.Sprintf("add (%s).%s call in %s", cachePkgName, cachePurgeFuncName, initializeFuncName),
			TextEdits: []analysis.TextEdit{{
				Pos:     pos,
				End:     end,
				NewText: buf.Bytes(),
			}},
		}},
	})

	return importPkgs, nil
}
