package main

import (
	"container/list"
	"flag"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
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

	funcs, err := buildFuncs(fset, pkgs, ssaProgram)
	if err != nil {
		return fmt.Errorf("failed to build funcs: %w", err)
	}

	nodes := buildGraph(funcs)

	mermaid, err := writeMermaid(nodes)
	if err != nil {
		return fmt.Errorf("failed to write mermaid: %w", err)
	}
	println(mermaid)

	return nil
}

func buildFuncs(fset *token.FileSet, pkgs []*packages.Package, ssaProgram *ssa.Program) ([]function, error) {
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
				var calls []string
				for _, block := range ssaFunc.Blocks {
					for _, instr := range block.Instrs {
						switch instr := instr.(type) {
						case *ssa.BinOp:
							var pos token.Position
							for _, val := range []interface{ Pos() token.Pos }{instr.X, instr, def} {
								if val == nil {
									continue
								}

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
								if val == nil {
									continue
								}

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
								if val == nil {
									continue
								}

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
								if val == nil {
									continue
								}

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
									if val == nil {
										continue
									}

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
								if val == nil {
									continue
								}

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
								if val == nil {
									continue
								}

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
							if f, ok := instr.Call.Value.(*ssa.Function); ok {
								calls = append(calls, f.Object().Id())
							}

							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Common(), instr, def} {
									if val == nil {
										continue
									}

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
							if f, ok := instr.Call.Value.(*ssa.Function); ok {
								calls = append(calls, f.Object().Id())
							}

							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Common(), instr, def} {
									if val == nil {
										continue
									}

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
							if f, ok := instr.Call.Value.(*ssa.Function); ok {
								calls = append(calls, f.Object().Id())
							}

							for _, arg := range instr.Call.Args {
								var pos token.Position
								for _, val := range []interface{ Pos() token.Pos }{arg, instr.Common(), instr, def} {
									if val == nil {
										continue
									}

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

				if len(queries) == 0 && len(calls) == 0 {
					continue
				}

				funcName := strings.TrimPrefix(strings.Replace(def.FullName(), pkg.Module.Path, "", 1), ".")
				funcs = append(funcs, function{
					id:      def.Id(),
					name:    funcName,
					queries: queries,
					calls:   calls,
				})
			}
		}
	}

	return funcs, nil
}

type function struct {
	id      string
	name    string
	queries []query
	calls   []string
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

type node struct {
	id       string
	label    string
	nodeType nodeType
	edges    []edge
}

type nodeType uint8

const (
	nodeTypeUnknown nodeType = iota
	nodeTypeTable
	nodeTypeFunction
)

type edge struct {
	label    string
	node     *node
	edgeType edgeType
}

type edgeType uint8

const (
	edgeTypeUnknown edgeType = iota
	edgeTypeInsert
	edgeTypeUpdate
	edgeTypeDelete
	edgeTypeSelect
	edgeTypeCall
)

func buildGraph(funcs []function) []*node {
	type tmpEdge struct {
		label    string
		edgeType edgeType
		childID  string
	}
	type tmpNode struct {
		*node
		edges []tmpEdge
	}
	tmpNodeMap := make(map[string]tmpNode, len(funcs))
	for _, f := range funcs {
		var edges []tmpEdge
		for _, q := range f.queries {
			id := tableID(q.table)
			tmpNodeMap[id] = tmpNode{
				node: &node{
					id:       id,
					label:    q.table,
					nodeType: nodeTypeTable,
				},
			}

			var edgeType edgeType
			switch q.queryType {
			case queryTypeSelect:
				edgeType = edgeTypeSelect
			case queryTypeInsert:
				edgeType = edgeTypeInsert
			case queryTypeUpdate:
				edgeType = edgeTypeUpdate
			case queryTypeDelete:
				edgeType = edgeTypeDelete
			default:
				log.Printf("unknown query type: %v\n", q.queryType)
				continue
			}

			edges = append(edges, tmpEdge{
				label:    "",
				edgeType: edgeType,
				childID:  tableID(q.table),
			})
		}

		for _, c := range f.calls {
			id := funcID(c)
			edges = append(edges, tmpEdge{
				label:    "",
				edgeType: edgeTypeCall,
				childID:  id,
			})
		}

		slices.SortFunc(edges, func(a, b tmpEdge) int {
			switch {
			case a.childID < b.childID:
				return -1
			case a.childID > b.childID:
				return 1
			default:
				return 0
			}
		})
		edges = slices.Compact(edges)

		id := funcID(f.id)
		tmpNodeMap[id] = tmpNode{
			node: &node{
				id:       id,
				label:    f.name,
				nodeType: nodeTypeFunction,
			},
			edges: edges,
		}
	}

	type revEdge struct {
		label    string
		edgeType edgeType
		parentID string
	}
	revEdgeMap := make(map[string][]revEdge)
	for _, tmpNode := range tmpNodeMap {
		for _, tmpEdge := range tmpNode.edges {
			revEdgeMap[tmpEdge.childID] = append(revEdgeMap[tmpEdge.childID], revEdge{
				label:    tmpEdge.label,
				edgeType: tmpEdge.edgeType,
				parentID: tmpNode.id,
			})
		}
	}

	newNodeMap := make(map[string]tmpNode, len(tmpNodeMap))
	nodeQueue := list.New()
	for id, node := range tmpNodeMap {
		if node.nodeType == nodeTypeTable {
			newNodeMap[id] = node
			nodeQueue.PushBack(node)
			delete(tmpNodeMap, id)
			continue
		}
	}

	for {
		element := nodeQueue.Front()
		if element == nil {
			break
		}
		nodeQueue.Remove(element)

		node := element.Value.(tmpNode)
		for _, edge := range revEdgeMap[node.id] {
			parent := tmpNodeMap[edge.parentID]
			newNodeMap[edge.parentID] = parent
			nodeQueue.PushBack(parent)
		}
		delete(revEdgeMap, node.id)
	}

	var nodes []*node
	for _, tmpNode := range newNodeMap {
		node := tmpNode.node
		for _, tmpEdge := range tmpNode.edges {
			child, ok := newNodeMap[tmpEdge.childID]
			if !ok {
				continue
			}

			node.edges = append(node.edges, edge{
				label:    tmpEdge.label,
				node:     child.node,
				edgeType: tmpEdge.edgeType,
			})
		}
		nodes = append(nodes, node)
	}

	return nodes
}

func funcID(functionID string) string {
	return fmt.Sprintf("func:%s", functionID)
}

func tableID(table string) string {
	return fmt.Sprintf("table:%s", table)
}

const (
	mermaidHeader = "# DB Graph\n```mermaid\ngraph LR\n"
	mermaidFooter = "```"
)

func writeMermaid(nodes []*node) (string, error) {
	sb := &strings.Builder{}
	_, err := sb.WriteString(mermaidHeader)
	if err != nil {
		return "", fmt.Errorf("failed to write header: %w", err)
	}

	for _, node := range nodes {
		for _, edge := range node.edges {
			if edge.label == "" {
				_, err = sb.WriteString(fmt.Sprintf("  %s[%s] --> %s[%s]\n", node.id, node.label, edge.node.id, edge.node.label))
				if err != nil {
					return "", fmt.Errorf("failed to write edge: %w\n", err)
				}
			} else {
				_, err = sb.WriteString(fmt.Sprintf("  %s[%s] -- %s --> %s[%s]\n", node.id, node.label, edge.label, edge.node.id, edge.node.label))
				if err != nil {
					return "", fmt.Errorf("failed to write edge: %w\n", err)
				}
			}
		}
	}

	_, err = sb.WriteString(mermaidFooter)
	if err != nil {
		return "", fmt.Errorf("failed to write footer: %w", err)
	}

	return sb.String(), nil
}
