package gin

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"reflect"

	"github.com/gostaticanalysis/analysisutil"
	"github.com/mazrean/isucon-go-tools/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

const (
	ginPkgName         = "github.com/gin-gonic/gin"
	apiPkgName         = "github.com/mazrean/isucon-go-tools/api"
	apiPkgDefaultIdent = "api"
	apiFuncName        = "WrapGin"
)

var (
	ginFuncNames = []string{"New", "Default"}

	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "gin",
		Doc:        "automatically setup github.com/gin-gonic/gin package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
	}
)

func run(pass *analysis.Pass) (any, error) {
	var pkgIdent string
	for _, pkg := range pass.Pkg.Imports() {
		if analysisutil.RemoveVendor(pkg.Path()) == ginPkgName {
			pkgIdent = pkg.Name()
			break
		}
	}
	if len(pkgIdent) == 0 {
		return importPkgs, nil
	}

	callExprs := []*ast.CallExpr{}
	for _, f := range pass.Files {
		v := visitor{
			pkgIdent:  pkgIdent,
			funcNames: ginFuncNames,
		}

		ast.Walk(&v, f)

		if len(v.callExprs) != 0 {
			importPkgs = append(importPkgs, &suggest.ImportInfo{
				File: f,
				Path: apiPkgName,
			})

			callExprs = append(callExprs, v.callExprs...)
		}
	}

	if len(callExprs) == 0 {
		return importPkgs, nil
	}

	for _, callExpr := range callExprs {
		buf := bytes.Buffer{}

		err := format.Node(&buf, pass.Fset, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(apiPkgDefaultIdent),
				Sel: ast.NewIdent(apiFuncName),
			},
			Args: []ast.Expr{callExpr},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callExpr.Pos(),
			Message: fmt.Sprintf("should wrap *(%s).%s with (%s).%s", ginPkgName, "Engin", apiPkgName, apiFuncName),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("wrap *(%s).%s with (%s).%s", ginPkgName, "Engin", apiPkgName, apiFuncName),
				TextEdits: []analysis.TextEdit{{
					Pos:     callExpr.Pos(),
					End:     callExpr.End(),
					NewText: buf.Bytes(),
				}},
			}},
		})
	}

	return importPkgs, nil
}

type visitor struct {
	pkgIdent  string
	funcNames []string
	callExprs []*ast.CallExpr
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch expr := node.(type) {
	case *ast.CallExpr:
		calleeSelector, ok := expr.Fun.(*ast.SelectorExpr)
		if !ok {
			return v
		}

		selName, ok := calleeSelector.X.(*ast.Ident)
		if !ok {
			return v
		}

		if selName.Name != v.pkgIdent {
			return v
		}

		for _, funcName := range v.funcNames {
			if calleeSelector.Sel.Name == funcName {
				v.callExprs = append(v.callExprs, expr)
				break
			}
		}

		return v
	}

	return v
}
