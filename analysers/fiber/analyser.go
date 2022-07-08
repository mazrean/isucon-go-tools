package fiber

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
	fiberPkgName       = "github.com/gofiber/fiber/v2"
	fiberFuncName      = "New"
	apiPkgName         = "github.com/mazrean/isucon-go-tools/http"
	apiPkgDefaultIdent = "isuhttp"
	apiFuncName        = "FiberNew"
)

var (
	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "fiber",
		Doc:        "automatically setup github.com/gofiber/fiber/v2 package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
	}
)

func run(pass *analysis.Pass) (any, error) {
	var pkgIdent string
	for _, pkg := range pass.Pkg.Imports() {
		if analysisutil.RemoveVendor(pkg.Path()) == fiberPkgName {
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
			pkgIdent: pkgIdent,
			funcName: fiberFuncName,
		}

		ast.Walk(&v, f)

		if len(v.callExprs) != 0 {
			importPkgs = append(importPkgs, &suggest.ImportInfo{
				File:  f,
				Ident: apiPkgDefaultIdent,
				Path:  apiPkgName,
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
			Args: callExpr.Args,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callExpr.Pos(),
			Message: fmt.Sprintf("should replace (%s).%s with (%s).%s", fiberPkgName, fiberFuncName, apiPkgName, apiFuncName),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("replace (%s).%s with (%s).%s", fiberPkgName, fiberFuncName, apiPkgName, apiFuncName),
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
	funcName  string
	callExprs []*ast.CallExpr
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

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

		if selName.Name == apiPkgDefaultIdent && calleeSelector.Sel.Name == apiFuncName {
			return nil
		}

		if selName.Name != v.pkgIdent {
			return v
		}

		if calleeSelector.Sel.Name != v.funcName {
			return v
		}

		v.callExprs = append(v.callExprs, expr)

		return v
	}

	return v
}
