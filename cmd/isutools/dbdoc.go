package main

import (
	"flag"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	dbDocFlagSet = flag.NewFlagSet("dbdoc", flag.ExitOnError)
	dst          string
	wd           string
)

func init() {
	dbDocFlagSet.StringVar(&dst, "dst", "./isudoc", "destination directory")
}

func dbDoc(args []string) error {
	err := dbDocFlagSet.Parse(args)
	if err != nil {
		return fmt.Errorf("failed to parse flag: %w", err)
	}

	wd, err = os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fset := token.NewFileSet()
	pkgs, err := packages.Load(&packages.Config{
		Fset: fset,
		Mode: packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedImports | packages.NeedTypesInfo | packages.NeedName | packages.NeedModule,
	}, dbDocFlagSet.Args()...)
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	ssaProgram, _ := ssautil.AllPackages(pkgs, ssa.BareInits)
	ssaProgram.Build()

	var funcs []function
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

				var queries []query
				for _, block := range ssaFunc.Blocks {
					for _, instr := range block.Instrs {
						switch instr := instr.(type) {
						case *ssa.BinOp:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr.X, instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok := newQueryFromValue(pos, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)

							for _, val := range []interface{ Pos() token.Pos }{instr.Y, instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok = newQueryFromValue(pos, instr.Y)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.ChangeType:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok := newQueryFromValue(pos, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Convert:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr.X, instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok := newQueryFromValue(pos, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.MakeClosure:
							for _, bind := range instr.Bindings {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{bind, instr, def} {
									pos = fset.Position(val.Pos())
									if pos.IsValid() {
										break
									}
								}
								query, ok := newQueryFromValue(pos, bind)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.MultiConvert:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr.X, instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok := newQueryFromValue(pos, instr.X)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Store:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr.Val, instr, def} {
								pos = fset.Position(val.Pos())
								if pos.IsValid() {
									break
								}
							}
							query, ok := newQueryFromValue(pos, instr.Val)
							if !ok {
								continue
							}

							queries = append(queries, query)
						case *ssa.Call:
							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Value(), instr, def} {
									pos = fset.Position(val.Pos())
									if pos.IsValid() {
										break
									}
								}
								query, ok := newQueryFromValue(pos, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.Defer:
							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Value(), instr, def} {
									pos = fset.Position(val.Pos())
									if pos.IsValid() {
										break
									}
								}
								query, ok := newQueryFromValue(pos, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						case *ssa.Go:
							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Value(), instr, def} {
									pos = fset.Position(val.Pos())
									if pos.IsValid() {
										break
									}
								}
								query, ok := newQueryFromValue(pos, arg)
								if !ok {
									continue
								}

								queries = append(queries, query)
							}
						}
					}
				}

				if len(queries) == 0 {
					continue
				}

				funcName := strings.Replace(def.FullName(), pkg.Module.Path, "", 1)
				funcs = append(funcs, function{
					name:    funcName,
					queries: queries,
				})
			}
		}
	}

	for _, f := range funcs {
		fmt.Println(f)
	}

	return nil
}

type function struct {
	name    string
	queries []query
}

type queryType uint8

const (
	queryTypeSelect queryType = iota + 1
	queryTypeInsert
	queryTypeUpdate
	queryTypeDelete
)

func (qt queryType) String() string {
	switch qt {
	case queryTypeSelect:
		return "select"
	case queryTypeInsert:
		return "insert"
	case queryTypeUpdate:
		return "update"
	case queryTypeDelete:
		return "delete"
	}

	return ""
}

type query struct {
	queryType queryType
	table     string
	pos       token.Position
}

func newQueryFromValue(pos token.Position, v ssa.Value) (query, bool) {
	strQuery, ok := checkValue(v, pos)
	if !ok {
		return query{}, false
	}

	q, ok := analyzeSQL(strQuery)
	if !ok {
		return query{}, false
	}

	fmt.Printf("%s(%s): %s\n", q.queryType, q.table, strQuery.value)

	return q, true
}

type stringLiteral struct {
	value string
	pos   token.Position
}

func checkValue(v ssa.Value, pos token.Position) (*stringLiteral, bool) {
	constValue, ok := v.(*ssa.Const)
	if !ok || constValue == nil || constValue.Value == nil {
		return nil, false
	}

	if constValue.Value.Kind() != constant.String {
		return nil, false
	}

	return &stringLiteral{
		value: constant.StringVal(constValue.Value),
		pos:   pos,
	}, true
}

var (
	selectRe = regexp.MustCompile("^select\\s+.*\\s+from\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	insertRe = regexp.MustCompile("^insert\\s+into\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	updateRe = regexp.MustCompile("^update\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
	deleteRe = regexp.MustCompile("^delete\\s+from\\s+[\\[\"'`]?(\\w+)[\\]\"'`]?\\s*")
)

func analyzeSQL(sql *stringLiteral) (query, bool) {
	sqlValue := strings.ToLower(sql.value)

	switch {
	case strings.HasPrefix(sqlValue, "select"):
		matches := selectRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableName, ok := tableForm(sql)
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
			tableName, ok := tableForm(sql)
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
			tableName, ok := tableForm(sql)
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
			tableName, ok := tableForm(sql)
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

func tableForm(sql *stringLiteral) (string, bool) {
	filename, err := filepath.Rel(wd, sql.pos.Filename)
	if err != nil {
		log.Printf("failed to get relative path: %v", err)
		return "", false
	}

	fmt.Printf("table name(%s:%d:%d): ", filename, sql.pos.Line, sql.pos.Column)
	var tableName string
	_, err = fmt.Scanln(&tableName)
	if err != nil {
		return "", false
	}

	if tableName == "" {
		return "", false
	}

	return tableName, true
}
