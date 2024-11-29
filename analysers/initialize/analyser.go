package initialize

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
	isutoolsPkgName          = "github.com/mazrean/isucon-go-tools/v2"
	isutoolsPkgDefaultIdent  = "isutools"
	initializeBeforeFuncName = "BeforeInitialize"
	initializeAfterFuncName  = "AfterInitialize"
	initializeKeyword        = "initialize"
)

var (
	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "initialize",
		Doc:        "automatically setup initialize tasks",
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
			if !ok || funcDecl == nil || funcDecl.Body == nil {
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

	beforeCalled := false
	afterCalled := false
	ast.Inspect(initializeFuncDecl.Body, func(n ast.Node) bool {
		if beforeCalled && afterCalled {
			return false
		}

		switch n := n.(type) {
		case *ast.CallExpr:
			if n == nil || n.Fun == nil {
				return true
			}

			selectorExpr, ok := n.Fun.(*ast.SelectorExpr)
			if !ok || selectorExpr == nil {
				return true
			}

			pkgIdent, ok := selectorExpr.X.(*ast.Ident)
			if !ok || pkgIdent == nil {
				return true
			}

			if pkgIdent.Name == isutoolsPkgDefaultIdent && selectorExpr.Sel.Name == initializeBeforeFuncName {
				beforeCalled = true
				return false
			}
		case *ast.DeferStmt:
			if n == nil || n.Call == nil || n.Call.Fun == nil {
				return true
			}

			selectorExpr, ok := n.Call.Fun.(*ast.SelectorExpr)
			if !ok || selectorExpr == nil {
				return true
			}

			pkgIdent, ok := selectorExpr.X.(*ast.Ident)
			if !ok || pkgIdent == nil {
				return true
			}

			if pkgIdent.Name == isutoolsPkgDefaultIdent && selectorExpr.Sel.Name == initializeAfterFuncName {
				afterCalled = true
				return false
			}
		}

		return true
	})

	if beforeCalled && afterCalled {
		return importPkgs, nil
	}

	importPkgs = append(importPkgs, &suggest.ImportInfo{
		File:  initializeFuncFile,
		Ident: isutoolsPkgDefaultIdent,
		Path:  isutoolsPkgName,
	})

	buf := bytes.Buffer{}

	var list []ast.Stmt
	if initializeFuncDecl.Body == nil {
		list = make([]ast.Stmt, 0, 1)
	} else {
		list = make([]ast.Stmt, 0, len(initializeFuncDecl.Body.List)+2)
	}
	list = append(list, &ast.ExprStmt{
		X: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(isutoolsPkgDefaultIdent),
				Sel: ast.NewIdent(initializeBeforeFuncName),
			},
			Args: []ast.Expr{},
		},
	}, &ast.DeferStmt{
		Call: &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(isutoolsPkgDefaultIdent),
				Sel: ast.NewIdent(initializeAfterFuncName),
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
		Message: fmt.Sprintf("%s should call (%s).%s and (%s).%s", initializeFuncName, isutoolsPkgName, initializeBeforeFuncName, isutoolsPkgName, initializeAfterFuncName),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: fmt.Sprintf("add (%s).%s and (%s).%s call in %s", isutoolsPkgName, initializeBeforeFuncName, isutoolsPkgName, initializeAfterFuncName, initializeFuncName),
			TextEdits: []analysis.TextEdit{{
				Pos:     pos,
				End:     end,
				NewText: buf.Bytes(),
			}},
		}},
	})

	return importPkgs, nil
}
