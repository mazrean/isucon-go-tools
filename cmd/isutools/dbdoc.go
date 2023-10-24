package main

import (
	"flag"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	dbDocFlagSet = flag.NewFlagSet("dbdoc", flag.ExitOnError)
	dst          string
)

func init() {
	dbDocFlagSet.StringVar(&dst, "dst", "./isudoc", "destination directory")
}

func dbDoc(args []string) error {
	err := dbDocFlagSet.Parse(args)
	if err != nil {
		return fmt.Errorf("failed to parse flag: %w", err)
	}

	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Fset: fset,
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedImports | packages.NeedTypesInfo,
	}, dbDocFlagSet.Args()...)
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	ssaProgram, _ := ssautil.AllPackages(pkgs, ssa.BuildSerially|ssa.SanityCheckFunctions|ssa.GlobalDebug|ssa.BareInits|ssa.NaiveForm)
	ssaProgram.Build()

	var queries []query
	for _, pkg := range pkgs {
		for _, def := range pkg.TypesInfo.Defs {
			if def == nil {
				continue
			}

			switch def := def.(type) {
			case *types.Func:
				ssaFunc := ssaProgram.FuncValue(def)
				if ssaFunc == nil {
					continue
				}

				for _, block := range ssaFunc.Blocks {
					for _, instr := range block.Instrs {
						switch instr := instr.(type) {
						case *ssa.BinOp:
							query, ok := newQueryFromValue(fset, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)

							query, ok = newQueryFromValue(fset, instr.Y)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.ChangeType:
							query, ok := newQueryFromValue(fset, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Convert:
							query, ok := newQueryFromValue(fset, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.MakeClosure:
							for _, bind := range instr.Bindings {
								query, ok := newQueryFromValue(fset, bind)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.MultiConvert:
							query, ok := newQueryFromValue(fset, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Store:
							query, ok := newQueryFromValue(fset, instr.Val)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Call:
							for _, arg := range instr.Call.Args {
								query, ok := newQueryFromValue(fset, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.Defer:
							for _, arg := range instr.Call.Args {
								query, ok := newQueryFromValue(fset, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.Go:
							for _, arg := range instr.Call.Args {
								query, ok := newQueryFromValue(fset, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						}
					}
				}
			}
		}
	}

	for _, query := range queries {
		fmt.Println(query)
	}

	return nil
}

type queryType uint8

const (
	queryTypeSelect queryType = iota + 1
	queryTypeInsert
	queryTypeUpdate
	queryTypeDelete
)

type query struct {
	queryType queryType
	table     string
	pos       token.Pos
}

func newQueryFromValue(fset *token.FileSet, v ssa.Value) (query, bool) {
	strQuery, ok := checkValue(v)
	if !ok {
		return query{}, false
	}

	return analyseSQL(fset, strQuery)
}

type stringLiteral struct {
	value string
	pos   token.Pos
}

func checkValue(v ssa.Value) (*stringLiteral, bool) {
	constValue, ok := v.(*ssa.Const)
	if !ok || constValue == nil || constValue.Value == nil {
		return nil, false
	}

	if constValue.Value.Kind() != constant.String {
		return nil, false
	}

	return &stringLiteral{
		value: constant.StringVal(constValue.Value),
		pos:   constValue.Pos(),
	}, true
}

var (
	selectRe = regexp.MustCompile("^select\\s+.*\\s+from\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	insertRe = regexp.MustCompile("^insert\\s+into\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	updateRe = regexp.MustCompile("^update\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	deleteRe = regexp.MustCompile("^delete\\s+from\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
)

func analyseSQL(fset *token.FileSet, sql *stringLiteral) (query, bool) {
	sqlValue := strings.ToLower(sql.value)

	switch {
	case strings.HasPrefix(sqlValue, "select"):
		matches := selectRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableName, ok := tableForm(fset, sql.pos)
			if !ok {
				return query{}, false
			}

			return query{
				queryType: queryTypeSelect,
				table:     tableName,
				pos:       sql.pos,
			}, true
		}

		return query{
			queryType: queryTypeSelect,
			table:     matches[1],
			pos:       sql.pos,
		}, true
	case strings.HasPrefix(sqlValue, "insert"):
		matches := insertRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableName, ok := tableForm(fset, sql.pos)
			if !ok {
				return query{}, false
			}

			return query{
				queryType: queryTypeInsert,
				table:     tableName,
				pos:       sql.pos,
			}, true
		}

		return query{
			queryType: queryTypeInsert,
			table:     matches[1],
			pos:       sql.pos,
		}, true
	case strings.HasPrefix(sqlValue, "update"):
		matches := updateRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableName, ok := tableForm(fset, sql.pos)
			if !ok {
				return query{}, false
			}

			return query{
				queryType: queryTypeUpdate,
				table:     tableName,
				pos:       sql.pos,
			}, true
		}

		return query{
			queryType: queryTypeUpdate,
			table:     matches[1],
			pos:       sql.pos,
		}, true
	case strings.HasPrefix(sqlValue, "delete"):
		matches := deleteRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableName, ok := tableForm(fset, sql.pos)
			if !ok {
				return query{}, false
			}

			return query{
				queryType: queryTypeDelete,
				table:     tableName,
				pos:       sql.pos,
			}, true
		}

		return query{
			queryType: queryTypeDelete,
			table:     matches[1],
			pos:       sql.pos,
		}, true
	}

	return query{}, false
}

func tableForm(fset *token.FileSet, pos token.Pos) (string, bool) {
	position := fset.Position(pos)
	fmt.Printf("table name of %s:%d:%d :", position.Filename, position.Line, position.Column)
	var tableName string
	_, err := fmt.Scan(&tableName)
	if err != nil {
		return "", false
	}

	if tableName == "" {
		return "", false
	}

	return tableName, true
}
