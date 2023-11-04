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
	"strconv"
	"strings"

	"github.com/mazrean/isucon-go-tools/pkg/analyze"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

var (
	dbDocFlagSet   = flag.NewFlagSet("dbdoc", flag.ExitOnError)
	dst            string
	wd             string
	ignores        sliceString
	ignorePrefixes sliceString
)

type sliceString []string

func (ss *sliceString) String() string {
	return fmt.Sprintf("%s", *ss)
}

func (ss *sliceString) Set(value string) error {
	*ss = append(*ss, value)
	return nil
}

func init() {
	dbDocFlagSet.StringVar(&dst, "dst", "./dbdoc.md", "destination file")
	dbDocFlagSet.Var(&ignores, "ignore", "ignore function")
	dbDocFlagSet.Var(&ignorePrefixes, "ignorePrefix", "ignore function")
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

	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to make directory: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(mermaid)
	if err != nil {
		return fmt.Errorf("failed to write mermaid: %w", err)
	}

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

				queries, calls := analyzeFuncBody(ssaFunc.Blocks, def, fset)

				for _, anonFunc := range ssaFunc.AnonFuncs {
					var def poser = anonFunc
					if !anonFunc.Pos().IsValid() {
						def = ssaFunc
					}

					anonQueries, anonCalls := analyzeFuncBody(anonFunc.Blocks, def, fset)
					queries = append(queries, anonQueries...)
					calls = append(calls, anonCalls...)
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

type poser interface {
	Pos() token.Pos
}

func analyzeFuncBody(blocks []*ssa.BasicBlock, def poser, fset *token.FileSet) ([]query, []string) {
	var queries []query
	var calls []string
	for _, block := range blocks {
		for _, instr := range block.Instrs {
			switch instr := instr.(type) {
			case *ssa.BinOp:
				var pos token.Position
				for _, val := range []poser{instr.X, instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries := newQueryFromValue(pos, instr.X)
				queries = append(queries, newQueries...)

				for _, val := range []poser{instr.Y, instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries = newQueryFromValue(pos, instr.Y)
				queries = append(queries, newQueries...)
			case *ssa.ChangeType:
				var pos token.Position
				for _, val := range []poser{instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries := newQueryFromValue(pos, instr.X)
				queries = append(queries, newQueries...)
			case *ssa.Convert:
				var pos token.Position
				for _, val := range []poser{instr.X, instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries := newQueryFromValue(pos, instr.X)
				queries = append(queries, newQueries...)
			case *ssa.MakeClosure:
				for _, bind := range instr.Bindings {
					var pos token.Position
					for _, val := range []poser{bind, instr, def} {
						if val == nil {
							continue
						}

						pos = fset.Position(val.Pos())
						if pos.IsValid() {
							break
						}
					}
					newQueries := newQueryFromValue(pos, bind)
					queries = append(queries, newQueries...)
				}
			case *ssa.MultiConvert:
				var pos token.Position
				for _, val := range []poser{instr.X, instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries := newQueryFromValue(pos, instr.X)
				queries = append(queries, newQueries...)
			case *ssa.Store:
				var pos token.Position
				for _, val := range []poser{instr.Val, instr, def} {
					if val == nil {
						continue
					}

					pos = fset.Position(val.Pos())
					if pos.IsValid() {
						break
					}
				}
				newQueries := newQueryFromValue(pos, instr.Val)
				queries = append(queries, newQueries...)
			case *ssa.Call:
				if f, ok := instr.Call.Value.(*ssa.Function); ok {
					if f.Object() == nil {
						continue
					}
					calls = append(calls, f.Object().Id())
				}

				for _, arg := range instr.Call.Args {
					var pos token.Position
					for _, val := range []poser{arg, instr.Common(), instr, def} {
						if val == nil {
							continue
						}

						pos = fset.Position(val.Pos())
						if pos.IsValid() {
							break
						}
					}
					newQueries := newQueryFromValue(pos, arg)
					queries = append(queries, newQueries...)
				}
			case *ssa.Defer:
				if f, ok := instr.Call.Value.(*ssa.Function); ok {
					if f.Object() == nil {
						continue
					}
					calls = append(calls, f.Object().Id())
				}

				for _, arg := range instr.Call.Args {
					var pos token.Position
					for _, val := range []poser{arg, instr.Common(), instr, def} {
						if val == nil {
							continue
						}

						pos = fset.Position(val.Pos())
						if pos.IsValid() {
							break
						}
					}
					newQueries := newQueryFromValue(pos, arg)
					queries = append(queries, newQueries...)
				}
			case *ssa.Go:
				if f, ok := instr.Call.Value.(*ssa.Function); ok {
					if f.Object() == nil {
						continue
					}
					calls = append(calls, f.Object().Id())
				}

				for _, arg := range instr.Call.Args {
					var pos token.Position
					for _, val := range []poser{arg, instr.Common(), instr, def} {
						if val == nil {
							continue
						}

						pos = fset.Position(val.Pos())
						if pos.IsValid() {
							break
						}
					}
					newQueries := newQueryFromValue(pos, arg)
					queries = append(queries, newQueries...)
				}
			}
		}
	}

	return queries, calls
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

func newQueryFromValue(pos token.Position, v ssa.Value) []query {
	strQuery, ok := checkValue(v, pos)
	if !ok {
		return nil
	}

	queries := analyzeSQL(strQuery)

	for _, q := range queries {
		fmt.Printf("%s(%s): %s\n", q.queryType, q.table, strQuery.value)
	}

	return queries
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
	tableRe        = regexp.MustCompile("^\\s*[\\[\"'`]?(?P<Table>\\w+)[\\]\"'`]?\\s*")
	insertRe       = regexp.MustCompile("^insert\\s+into\\s+[\\[\"'`]?(?P<Table>\\w+)[\\]\"'`]?\\s*")
	deleteRe       = regexp.MustCompile("^delete\\s+from\\s+[\\[\"'`]?(?P<Table>\\w+)[\\]\"'`]?\\s*")
	selectKeywords = []string{" where ", " group by ", " having ", " window ", " order by ", "limit ", " for "}
)

func analyzeSQL(sql *stringLiteral) []query {
	sqlValue := strings.ToLower(sql.value)

	strQueries := extractSubQueries(sqlValue)

	var queries []query
	for _, sqlValue := range strQueries {
		newQueries := analyzeSQLWithoutSubQuery(sqlValue, sql)
		queries = append(queries, newQueries...)
	}

	return queries
}

type subQuery struct {
	query        string
	bracketCount uint
}

var (
	subQueryPrefixRe = regexp.MustCompile(`^\s*\(\s*select\s+`)
)

func extractSubQueries(sql string) []string {
	var subQueries []string

	rootQuery := ""
	var subQueryStack []subQuery
	for i := 0; i < len(sql); i++ {
		r := sql[i]
		switch r {
		case '(':
			match := subQueryPrefixRe.FindString(sql[i:])
			if len(match) != 0 {
				subQueryStack = append(subQueryStack, subQuery{
					query:        match,
					bracketCount: 0,
				})
				i += len(match)
				continue
			}

			if len(subQueryStack) == 0 {
				rootQuery += string(r)
				continue
			}

			subQueryStack[len(subQueryStack)-1].bracketCount++
			subQueryStack[len(subQueryStack)-1].query += string(r)
		case ')':
			if len(subQueryStack) == 0 {
				rootQuery += string(r)
				continue
			}

			if subQueryStack[len(subQueryStack)-1].bracketCount == 0 {
				subQueries = append(subQueries, subQueryStack[len(subQueryStack)-1].query)
				subQueryStack = subQueryStack[:len(subQueryStack)-1]
				continue
			}

			subQueryStack[len(subQueryStack)-1].bracketCount--
			subQueryStack[len(subQueryStack)-1].query += string(r)
		default:
			if len(subQueryStack) == 0 {
				rootQuery += string(r)
				continue
			}

			subQueryStack[len(subQueryStack)-1].query += string(r)
		}
	}

	for _, subQuery := range subQueryStack {
		subQueries = append(subQueries, subQuery.query)
	}

	if rootQuery != "" {
		subQueries = append(subQueries, rootQuery)
	}

	return subQueries
}

func analyzeSQLWithoutSubQuery(sqlValue string, sql *stringLiteral) []query {
	var queries []query
	switch {
	case strings.HasPrefix(sqlValue, "select"):
		_, after, found := strings.Cut(sqlValue, " from ")
		if !found {
			tableNames := tableForm(sql, sqlValue)

			for _, tableName := range tableNames {
				queries = append(queries, query{
					queryType: queryTypeSelect,
					table:     tableName,
					pos:       sql.pos,
				})
			}
			break
		}

		tmpTableNames := strings.Split(after, ",")
		var tableNames []string
	TABLE_LOOP:
		for _, tableName := range tmpTableNames {
			tableNames = append(tableNames, strings.Split(tableName, " join ")...)

			for _, keyword := range selectKeywords {
				if strings.Contains(tableName, keyword) {
					break TABLE_LOOP
				}
			}
		}

		for _, tableName := range tableNames {
			matches := tableRe.FindStringSubmatch(tableName)
			if len(matches) == 0 {
				continue
			}

			for i, name := range tableRe.SubexpNames() {
				if name == "Table" {
					queries = append(queries, query{
						queryType: queryTypeSelect,
						table:     matches[i],
						pos:       sql.pos,
					})
				}
			}
		}
	case strings.HasPrefix(sqlValue, "insert"):
		matches := insertRe.FindStringSubmatch(sqlValue)
		if len(matches) < 2 {
			tableNames := tableForm(sql, sqlValue)

			for _, tableName := range tableNames {
				queries = append(queries, query{
					queryType: queryTypeInsert,
					table:     tableName,
					pos:       sql.pos,
				})
			}
			break
		}

		queries = append(queries, query{
			queryType: queryTypeInsert,
			table:     matches[1],
			pos:       sql.pos,
		})
	case strings.HasPrefix(sqlValue, "update"):
		afterUpdate := strings.TrimPrefix(sqlValue, "update ")
		before, _, found := strings.Cut(afterUpdate, " set ")
		if !found {
			before = afterUpdate
		}

		tmpTableNames := strings.Split(before, ",")
		var tableNames []string
		for _, tableName := range tmpTableNames {
			tableNames = append(tableNames, strings.Split(tableName, " join ")...)
		}

		for _, tableName := range tableNames {
			matches := tableRe.FindStringSubmatch(tableName)
			if len(matches) == 0 {
				continue
			}

			for i, name := range tableRe.SubexpNames() {
				if name == "Table" {
					queries = append(queries, query{
						queryType: queryTypeUpdate,
						table:     matches[i],
						pos:       sql.pos,
					})
				}
			}
		}
	case strings.HasPrefix(sqlValue, "delete"):
		matches := deleteRe.FindStringSubmatch(sqlValue)

		for i, name := range deleteRe.SubexpNames() {
			if name == "Table" {
				queries = append(queries, query{
					queryType: queryTypeDelete,
					table:     matches[i],
					pos:       sql.pos,
				})
			}
		}
	}

	return queries
}

func tableForm(sql *stringLiteral, sqlValue string) []string {
	filename, err := filepath.Rel(wd, sql.pos.Filename)
	if err != nil {
		log.Printf("failed to get relative path: %v", err)
		return nil
	}

	fmt.Printf("query:%s\n", sqlValue)
	fmt.Printf("position: %s:%d:%d\n", filename, sql.pos.Line, sql.pos.Column)
	fmt.Print("table name?: ")
	var input string
	_, err = fmt.Scanln(&input)
	if err != nil {
		return nil
	}

	if input == "" {
		return nil
	}

	tableNames := strings.Split(input, ",")

	return tableNames
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
FUNC_LOOP:
	for _, f := range funcs {
		if f.name == "main" || analyze.IsInitializeFuncName(f.name) {
			continue
		}

		for _, ignore := range ignores {
			if f.name == ignore {
				continue FUNC_LOOP
			}
		}

		for _, ignorePrefix := range ignorePrefixes {
			if strings.HasPrefix(f.name, ignorePrefix) {
				continue FUNC_LOOP
			}
		}

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
	mermaidHeader = "# DB Graph\n```mermaid\ngraph LR\n  classDef func fill:#1976D2,fill-opacity:0.5\n  classDef table fill:#795548,fill-opacity:0.5\n"
	mermaidFooter = "```"
)

func writeMermaid(nodes []*node) (string, error) {
	sb := &strings.Builder{}
	_, err := sb.WriteString(mermaidHeader)
	if err != nil {
		return "", fmt.Errorf("failed to write header: %w", err)
	}

	edgeID := 0
	var insertLinks, deleteLinks, selectLinks, updateLinks, callLinks []string
	for _, node := range nodes {
		var src string
		switch node.nodeType {
		case nodeTypeTable:
			src = fmt.Sprintf("%s[%s]:::table", node.id, node.label)
		case nodeTypeFunction:
			src = fmt.Sprintf("%s[%s]:::func", node.id, node.label)
		default:
			log.Printf("unknown node type: %v\n", node.nodeType)
			src = fmt.Sprintf("%s[%s]", node.id, node.label)
		}

		for _, edge := range node.edges {
			var dst, line string
			switch edge.node.nodeType {
			case nodeTypeTable:
				dst = fmt.Sprintf("%s[%s]:::table", edge.node.id, edge.node.label)
			case nodeTypeFunction:
				dst = fmt.Sprintf("%s[%s]:::func", edge.node.id, edge.node.label)
			default:
				log.Printf("unknown node type: %v\n", edge.node.nodeType)
				dst = fmt.Sprintf("%s[%s]", edge.node.id, edge.node.label)
			}

			line = "--"

			if edge.label == "" {
				_, err = sb.WriteString(fmt.Sprintf("  %s %s> %s\n", src, line, dst))
				if err != nil {
					return "", fmt.Errorf("failed to write edge: %w\n", err)
				}
			} else {
				_, err = sb.WriteString(fmt.Sprintf("  %s %s %s %s> %s\n", src, line, edge.label, line, dst))
				if err != nil {
					return "", fmt.Errorf("failed to write edge: %w\n", err)
				}
			}

			switch edge.edgeType {
			case edgeTypeInsert:
				insertLinks = append(insertLinks, strconv.Itoa(edgeID))
			case edgeTypeDelete:
				deleteLinks = append(deleteLinks, strconv.Itoa(edgeID))
			case edgeTypeSelect:
				selectLinks = append(selectLinks, strconv.Itoa(edgeID))
			case edgeTypeUpdate:
				updateLinks = append(updateLinks, strconv.Itoa(edgeID))
			case edgeTypeCall:
				callLinks = append(callLinks, strconv.Itoa(edgeID))
			default:
				log.Printf("unknown edge type: %v\n", edge.edgeType)
			}

			edgeID++
		}
	}

	if len(insertLinks) > 0 {
		_, err = sb.WriteString(fmt.Sprintf("  linkStyle %s stroke:#CDDC39,stroke-width:2px\n", strings.Join(insertLinks, ",")))
		if err != nil {
			return "", fmt.Errorf("failed to write link style: %w\n", err)
		}
	}
	if len(deleteLinks) > 0 {
		_, err = sb.WriteString(fmt.Sprintf("  linkStyle %s stroke:#F44336,stroke-width:2px\n", strings.Join(deleteLinks, ",")))
		if err != nil {
			return "", fmt.Errorf("failed to write link style: %w\n", err)
		}
	}
	if len(selectLinks) > 0 {
		_, err = sb.WriteString(fmt.Sprintf("  linkStyle %s stroke:#78909C,stroke-width:2px\n", strings.Join(selectLinks, ",")))
		if err != nil {
			return "", fmt.Errorf("failed to write link style: %w\n", err)
		}
	}
	if len(updateLinks) > 0 {
		_, err = sb.WriteString(fmt.Sprintf("  linkStyle %s stroke:#FF9800,stroke-width:2px\n", strings.Join(updateLinks, ",")))
		if err != nil {
			return "", fmt.Errorf("failed to write link style: %w\n", err)
		}
	}
	if len(callLinks) > 0 {
		_, err = sb.WriteString(fmt.Sprintf("  linkStyle %s stroke:#BBDEFB,stroke-width:2px\n", strings.Join(callLinks, ",")))
		if err != nil {
			return "", fmt.Errorf("failed to write link style: %w\n", err)
		}
	}

	_, err = sb.WriteString(mermaidFooter)
	if err != nil {
		return "", fmt.Errorf("failed to write footer: %w", err)
	}

	return sb.String(), nil
}
