package gin

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"reflect"

	"github.com/gostaticanalysis/analysisutil"
	"github.com/mazrean/isucon-go-tools/pkg/suggest"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

const (
	ginPkgName         = "github.com/gin-gonic/gin"
	ginEnginTypeName   = "Engine"
	apiPkgName         = "github.com/mazrean/isucon-go-tools/http"
	apiPkgDefaultIdent = "isuhttp"
	apiPrefix          = "Gin"
	apiFuncName        = "GinNew"
)

var (
	ginFuncNames   = []string{"New", "Default"}
	ginMethodNames = []string{"Run", "RunTLS"}

	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "gin",
		Doc:        "automatically setup github.com/gin-gonic/gin package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
		Requires:   []*analysis.Analyzer{buildssa.Analyzer},
	}
)

func run(pass *analysis.Pass) (any, error) {
	err := wrapNew(pass)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap gin.New: %w", err)
	}

	ssaGraph, ok := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	if !ok {
		return nil, errors.New("failed to get ssa graph")
	}

	enginType := analysisutil.TypeOf(pass, ginPkgName, ginEnginTypeName)
	if enginType == nil {
		return importPkgs, nil
	}

	funcTypes := make([]*types.Func, 0, len(ginMethodNames))
	for _, methodName := range ginMethodNames {
		funcType := analysisutil.MethodOf(enginType, methodName)
		if funcType == nil {
			continue
		}

		funcTypes = append(funcTypes, funcType)
	}

	callExprInfo, err := suggest.FindCallExpr(pass.Files, ssaGraph, funcTypes)
	if err != nil {
		return nil, fmt.Errorf("failed to find call expr: %w", err)
	}

	buf := bytes.Buffer{}

	for _, callExpr := range callExprInfo {
		importPkgs = append(importPkgs, &suggest.ImportInfo{
			File:  callExpr.File,
			Ident: apiPkgDefaultIdent,
			Path:  apiPkgName,
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
				Sel: ast.NewIdent(apiPrefix + callExpr.FuncType.Name()),
			},
			Args: args,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callExpr.Call.Pos(),
			Message: fmt.Sprintf("should replace %s with (%s).%s%s", callExpr.FuncType.FullName(), apiPkgName, apiPrefix, callExpr.FuncType.Name()),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("replace %s with (%s).%s%s", callExpr.FuncType.FullName(), apiPkgName, apiPrefix, callExpr.FuncType.Name()),
				TextEdits: []analysis.TextEdit{{
					Pos:     callExpr.Call.Pos(),
					End:     callExpr.Call.End(),
					NewText: buf.Bytes(),
				}},
			}},
		})

		buf.Reset()
	}

	return importPkgs, nil
}

func wrapNew(pass *analysis.Pass) error {
	var pkgIdent string
	for _, pkg := range pass.Pkg.Imports() {
		if analysisutil.RemoveVendor(pkg.Path()) == ginPkgName {
			pkgIdent = pkg.Name()
			break
		}
	}
	if len(pkgIdent) == 0 {
		return nil
	}

	funcInfoList := []*funcInfo{}
	for _, funcName := range ginFuncNames {
		funcInfoList = append(funcInfoList, &funcInfo{
			pkgName:  ginPkgName,
			pkgIdent: pkgIdent,
			funcName: funcName,
		})
	}

	callInfoList := []*callInfo{}
	for _, f := range pass.Files {
		v := visitor{
			funcInfoList: funcInfoList,
		}

		ast.Walk(&v, f)

		if len(v.callExprs) != 0 {
			importPkgs = append(importPkgs, &suggest.ImportInfo{
				File:  f,
				Ident: apiPkgDefaultIdent,
				Path:  apiPkgName,
			})

			callInfoList = append(callInfoList, v.callExprs...)
		}
	}

	if len(callInfoList) == 0 {
		return nil
	}

	for _, callInfo := range callInfoList {
		buf := bytes.Buffer{}

		err := format.Node(&buf, pass.Fset, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(apiPkgDefaultIdent),
				Sel: ast.NewIdent(apiFuncName),
			},
			Args: []ast.Expr{callInfo.expr},
		})
		if err != nil {
			return fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callInfo.expr.Pos(),
			Message: fmt.Sprintf("should wrap (%s).%s with (%s).%s", callInfo.funcInfo.pkgName, callInfo.funcInfo.funcName, apiPkgName, apiFuncName),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("wrap (%s).%s with (%s).%s", callInfo.funcInfo.pkgName, callInfo.funcInfo.funcName, apiPkgName, apiFuncName),
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

type funcInfo struct {
	pkgName  string
	pkgIdent string
	funcName string
}

type callInfo struct {
	funcInfo *funcInfo
	expr     *ast.CallExpr
}

type visitor struct {
	funcInfoList []*funcInfo
	callExprs    []*callInfo
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

		for _, funcInfo := range v.funcInfoList {
			if selName.Name == funcInfo.pkgIdent && calleeSelector.Sel.Name == funcInfo.funcName {
				v.callExprs = append(v.callExprs, &callInfo{
					funcInfo: funcInfo,
					expr:     expr,
				})
				break
			}
		}

		return v
	}

	return v
}
