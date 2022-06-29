package suggest

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type CallExprInfo struct {
	FuncType *types.Func
	File     *ast.File
	Call     *ast.CallExpr
}

type callInfo struct {
	funcType *types.Func
	pos      token.Pos
}

func FindCallExpr(files []*ast.File, ssaGraph *buildssa.SSA, funcTypes []*types.Func) ([]*CallExprInfo, error) {
	var callPosList []*callInfo
	for _, srcFn := range ssaGraph.SrcFuncs {
		for _, block := range srcFn.Blocks {
			for _, instr := range block.Instrs {
				instr, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}

				common := instr.Common()
				if common == nil {
					continue
				}

				callee := common.StaticCallee()
				if callee == nil {
					continue
				}

				calleeType, ok := callee.Object().(*types.Func)
				if !ok || calleeType == nil {
					continue
				}

				for _, fnType := range funcTypes {
					if fnType == calleeType {
						callPosList = append(callPosList, &callInfo{
							funcType: fnType,
							pos:      instr.Pos(),
						})
						break
					}
				}
			}
		}
	}

	visitor := newCallDetectVisitor(callPosList)
	for _, f := range files {
		visitor.SetFile(f)
		ast.Walk(visitor, f)
	}

	return *visitor.callExprs, nil
}

type callDetectVisitor struct {
	callPosList []*callInfo
	file        *ast.File
	callExprs   *[]*CallExprInfo
}

func newCallDetectVisitor(callPosList []*callInfo) *callDetectVisitor {
	sort.Slice(callPosList, func(i, j int) bool {
		return callPosList[i].pos < callPosList[j].pos
	})

	return &callDetectVisitor{
		callPosList: callPosList,
		callExprs:   &[]*CallExprInfo{},
	}
}

func (cdv *callDetectVisitor) SetFile(file *ast.File) {
	cdv.file = file
}

func (cdv *callDetectVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// node以下にある可能性のある、posの最小のindex
	i := sort.Search(len(cdv.callPosList), func(i int) bool {
		return cdv.callPosList[i].pos >= node.Pos()
	})
	// node以下にある可能性のある、posの最大のindex
	j := len(cdv.callPosList) - 1 - sort.Search(len(cdv.callPosList), func(i int) bool {
		return cdv.callPosList[len(cdv.callPosList)-1-i].pos < node.End()
	})
	if i > j {
		// 範囲内に探したいCallExprがないとき、子の探索はしない
		return nil
	}

	switch expr := node.(type) {
	case *ast.CallExpr:
		for _, callPos := range cdv.callPosList[i : j+1] {
			if callPos.pos == expr.Lparen {
				*cdv.callExprs = append(*cdv.callExprs, &CallExprInfo{
					FuncType: callPos.funcType,
					File:     cdv.file,
					Call:     expr,
				})
				break
			}
		}
	case *ast.GoStmt:
		for _, callPos := range cdv.callPosList[i : j+1] {
			if callPos.pos == expr.Go {
				*cdv.callExprs = append(*cdv.callExprs, &CallExprInfo{
					FuncType: callPos.funcType,
					File:     cdv.file,
					Call:     expr.Call,
				})
				break
			}
		}
	case *ast.DeferStmt:
		for _, callPos := range cdv.callPosList[i : j+1] {
			if callPos.pos == expr.Defer {
				*cdv.callExprs = append(*cdv.callExprs, &CallExprInfo{
					FuncType: callPos.funcType,
					File:     cdv.file,
					Call:     expr.Call,
				})
				break
			}
		}
	}

	return &callDetectVisitor{
		callPosList: cdv.callPosList[i : j+1],
		file:        cdv.file,
		callExprs:   cdv.callExprs,
	}
}
