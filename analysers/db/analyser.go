package db

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
	sqlPkgName        = "database/sql"
	sqlxPkgName       = "github.com/jmoiron/sqlx"
	dbPkgName         = "github.com/mazrean/isucon-go-tools/db"
	dbPkgDefaultIdent = "isudb"
	dbFuncName        = "DBMetricsSetup"
)

var (
	sqlFuncNames  = []string{"Open"}
	sqlxFuncNames = []string{"Open", "Connect"}

	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "db",
		Doc:        "automatically setup database/sql, github.com/jmoiron/sqlx package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
	}
)

func run(pass *analysis.Pass) (any, error) {
	var (
		sqlPkgIdent  string
		sqlxPkgIdent string
	)
	for _, pkg := range pass.Pkg.Imports() {
		if analysisutil.RemoveVendor(pkg.Path()) == sqlPkgName {
			sqlPkgIdent = pkg.Name()
		}

		if analysisutil.RemoveVendor(pkg.Path()) == sqlxPkgName {
			sqlxPkgIdent = pkg.Name()
		}
	}

	funcInfoList := []*funcInfo{}
	if len(sqlPkgIdent) != 0 {
		for _, funcName := range sqlFuncNames {
			funcInfoList = append(funcInfoList, &funcInfo{
				pkgName:  sqlPkgName,
				pkgIdent: sqlPkgIdent,
				funcName: funcName,
			})
		}
	}
	if len(sqlxPkgIdent) != 0 {
		for _, funcName := range sqlxFuncNames {
			funcInfoList = append(funcInfoList, &funcInfo{
				pkgName:  sqlxPkgIdent,
				pkgIdent: sqlxPkgIdent,
				funcName: funcName,
			})
		}
	}

	if len(funcInfoList) == 0 {
		return importPkgs, nil
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
				Ident: dbPkgDefaultIdent,
				Path:  dbPkgName,
			})

			callInfoList = append(callInfoList, v.callExprs...)
		}
	}

	if len(callInfoList) == 0 {
		return importPkgs, nil
	}

	for _, callInfo := range callInfoList {
		buf := bytes.Buffer{}

		err := format.Node(&buf, pass.Fset, &ast.CallExpr{
			Fun: &ast.SelectorExpr{
				X:   ast.NewIdent(dbPkgDefaultIdent),
				Sel: ast.NewIdent(dbFuncName),
			},
			Args: []ast.Expr{callInfo.expr.Fun},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to format import declaration: %w", err)
		}

		pass.Report(analysis.Diagnostic{
			Pos:     callInfo.expr.Fun.Pos(),
			Message: fmt.Sprintf("should wrap (%s).%s with (%s).%s", callInfo.funcInfo.pkgName, callInfo.funcInfo.funcName, dbPkgName, dbFuncName),
			SuggestedFixes: []analysis.SuggestedFix{{
				Message: fmt.Sprintf("wrap (%s).%s with (%s).%s", callInfo.funcInfo.pkgName, callInfo.funcInfo.funcName, dbPkgName, dbFuncName),
				TextEdits: []analysis.TextEdit{{
					Pos:     callInfo.expr.Fun.Pos(),
					End:     callInfo.expr.Fun.End(),
					NewText: buf.Bytes(),
				}},
			}},
		})
	}

	return importPkgs, nil
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
