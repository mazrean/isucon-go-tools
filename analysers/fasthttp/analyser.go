package fasthttp

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"reflect"

	"github.com/gostaticanalysis/analysisutil"
	"github.com/mazrean/isucon-go-tools/v2/pkg/suggest"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

const (
	fasthttpPkgName        = "github.com/valyala/fasthttp"
	fasthttpServerTypeName = "Server"
	apiPkgName             = "github.com/mazrean/isucon-go-tools/v2/http"
	apiPkgDefaultIdent     = "isuhttp"
	apiServerTypePrefix    = "Server"
	apiPrefix              = "Fast"
)

var (
	fasthttpFuncNames   = []string{"ListenAndServe", "ListenAndServeTLS", "ListenAndServeTLSEmbed"}
	fasthttpMethodNames = []string{"ListenAndServe", "ListenAndServeTLS", "ListenAndServeTLSEmbed"}

	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "fasthttp",
		Doc:        "automatically setup github.com/valyala/fasthttp package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
		Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	}
)

func run(pass *analysis.Pass) (any, error) {
	err := funcSetting(pass)
	if err != nil {
		return nil, fmt.Errorf("failed to set func: %w", err)
	}

	err = serverStructSetting(pass)
	if err != nil {
		return nil, fmt.Errorf("failed to set fasthttp.Server struct: %w", err)
	}

	return importPkgs, nil
}

func funcSetting(pass *analysis.Pass) error {
	var pkgIdent string
	for _, pkg := range pass.Pkg.Imports() {
		if analysisutil.RemoveVendor(pkg.Path()) == fasthttpPkgName {
			pkgIdent = pkg.Name()
			break
		}
	}
	if len(pkgIdent) == 0 {
		return nil
	}

	callInfos := []*callInfo{}
	for _, f := range pass.Files {
		v := visitor{
			pkgIdent:  pkgIdent,
			funcNames: fasthttpFuncNames,
		}

		ast.Walk(&v, f)

		if len(v.callExprs) != 0 {
			importPkgs = append(importPkgs, &suggest.ImportInfo{
				File:  f,
				Ident: apiPkgDefaultIdent,
				Path:  apiPkgName,
			})

			callInfos = append(callInfos, v.callExprs...)
		}
	}

	if len(callInfos) == 0 {
		return nil
	}

	for _, callInfo := range callInfos {
		buf := bytes.Buffer{}

		funcName := fmt.Sprintf("%s%s", apiPrefix, callInfo.funcName)
		err := format.Node(&buf, pass.Fset, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(apiPkgDefaultIdent),
				Sel: ast.NewIdent(funcName),
			},
			Args: callInfo.expr.Args,
		})
		if err != nil {
			return fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callInfo.expr.Pos(),
			Message: fmt.Sprintf("should replace (%s).%s with (%s).%s", fasthttpPkgName, callInfo.funcName, apiPkgName, funcName),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("replace (%s).%s with (%s).%s", fasthttpPkgName, callInfo.funcName, apiPkgName, funcName),
				TextEdits: []analysis.TextEdit{{
					Pos:     callInfo.expr.Pos(),
					End:     callInfo.expr.End(),
					NewText: buf.Bytes(),
				}},
			}},
		})
	}

	return nil
}

type visitor struct {
	pkgIdent  string
	funcNames []string
	callExprs []*callInfo
}

type callInfo struct {
	funcName string
	expr     *ast.CallExpr
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

		if selName.Name != v.pkgIdent {
			return v
		}

		for _, funcName := range v.funcNames {
			if calleeSelector.Sel.Name == funcName {
				v.callExprs = append(v.callExprs, &callInfo{
					funcName: funcName,
					expr:     expr,
				})

				break
			}
		}

		return v
	}

	return v
}

func serverStructSetting(pass *analysis.Pass) error {
	ssaGraph, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return errors.New("failed to get ssa graph")
	}

	enginType := analysisutil.TypeOf(pass, fasthttpPkgName, fasthttpServerTypeName)
	if enginType == nil {
		return nil
	}

	funcTypes := make([]*types.Func, 0, len(fasthttpMethodNames))
	for _, methodName := range fasthttpMethodNames {
		funcType := analysisutil.MethodOf(enginType, methodName)
		if funcType == nil {
			continue
		}

		funcTypes = append(funcTypes, funcType)
	}

	callExprInfo, err := suggest.FindCallExpr(pass.Files, ssaGraph, funcTypes)
	if err != nil {
		return fmt.Errorf("failed to find call expr: %w", err)
	}

	buf := bytes.Buffer{}

	for _, callExpr := range callExprInfo {
		importPkgs = append(importPkgs, &suggest.ImportInfo{
			File: callExpr.File,
			Path: apiPkgName,
		})

		selectorExpr, ok := callExpr.Call.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}

		args := make([]ast.Expr, 0, len(callExpr.Call.Args)+1)
		args = append(args, selectorExpr.X)
		args = append(args, callExpr.Call.Args...)

		err := format.Node(&buf, pass.Fset, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(apiPkgDefaultIdent),
				Sel: ast.NewIdent(apiPrefix + apiServerTypePrefix + callExpr.FuncType.Name()),
			},
			Args: args,
		})
		if err != nil {
			return fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callExpr.Call.Pos(),
			Message: fmt.Sprintf("should replace %s with (%s).%s%s%s", callExpr.FuncType.FullName(), apiPkgName, apiPrefix, apiServerTypePrefix, callExpr.FuncType.Name()),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("replace %s with (%s).%s%s%s", callExpr.FuncType.FullName(), apiPkgName, apiPrefix, apiServerTypePrefix, callExpr.FuncType.Name()),
				TextEdits: []analysis.TextEdit{{
					Pos:     callExpr.Call.Pos(),
					End:     callExpr.Call.End(),
					NewText: buf.Bytes(),
				}},
			}},
		})

		buf.Reset()
	}

	return nil
}
