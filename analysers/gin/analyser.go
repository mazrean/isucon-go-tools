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
	apiPkgName         = "github.com/mazrean/isucon-go-tools/api"
	apiPkgDefaultIdent = "api"
	apiPrefix          = "Gin"
)

var (
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
