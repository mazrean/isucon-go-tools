package isudb

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

type queryKey struct {
	driver     string
	normalized string
}

type queryExample struct {
	query     string
	args      []driver.Value
	namedArgs []driver.NamedValue
}

type queryInfo struct {
	ID         int          `json:"id"`
	Driver     string       `json:"driver"`
	Normalized string       `json:"normalized"`
	Example    queryExample `json:"-"`
	Latency    float64      `json:"latency"`
}

var (
	queryID        = &atomic.Uint64{}
	queryMapLocker = &sync.RWMutex{}
	queryMap       = map[queryKey]queryInfo{}
)

func init() {
	queryID.Store(0)
}

func queryExecHook(driver, normalizedQuery, rawQuery string, args []driver.Value, namedArgs []driver.NamedValue, latency float64) {
	key := queryKey{
		driver:     driver,
		normalized: normalizedQuery,
	}

	info, ok := func() (queryInfo, bool) {
		queryMapLocker.RLock()
		defer queryMapLocker.RUnlock()

		info, ok := queryMap[key]
		return info, ok
	}()

	if !ok {
		id := int(queryID.Add(1))

		func() {
			queryMapLocker.Lock()
			defer queryMapLocker.Unlock()

			queryMap[key] = queryInfo{
				ID:         id,
				Driver:     driver,
				Normalized: normalizedQuery,
				Example: queryExample{
					query:     rawQuery,
					args:      args,
					namedArgs: namedArgs,
				},
				Latency: latency,
			}
		}()
		return
	}

	// 同時に複数のクエリが実行される場合、latency最大のクエリとならないが、それなりに遅いクエリがとれれば良いため速度を優先
	func() {
		if info.Latency < latency {
			info.Example = queryExample{query: rawQuery, args: args}
			info.Latency = latency

			queryMapLocker.Lock()
			defer queryMapLocker.Unlock()

			queryMap[key] = info
		}
	}()
}

func queryListHandler(w http.ResponseWriter, r *http.Request) {
	queries := func() []queryInfo {
		queryMapLocker.RLock()
		defer queryMapLocker.RUnlock()

		queries := make([]queryInfo, 0, len(queryMap))
		for _, info := range queryMap {
			queries = append(queries, info)
		}
		return queries
	}()

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(queries)
}

type ExplainResult struct {
	Table        string   `json:"table"`
	PossibleKeys []string `json:"possible_keys"`
	Key          string   `json:"key"`
	Rows         int      `json:"rows"`
	Filtered     int      `json:"filtered"`
}

func queryExplainHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	query := func() *queryInfo {
		queryMapLocker.RLock()
		defer queryMapLocker.RUnlock()

		for _, info := range queryMap {
			if info.ID == id {
				return &info
			}
		}

		return nil
	}()
	if query == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	db, ok := dbMap[query.Driver]
	if !ok {
		http.Error(w, query.Driver+" driver not found", http.StatusInternalServerError)
		return
	}

	var explainResults []ExplainResult
	switch query.Driver {
	case "mysql":
		explainQuery := "EXPLAIN " + query.Example.query

		args := constructArgs(query.Example.args, query.Example.namedArgs)
		log.Printf("query: %s, args: %v", explainQuery, args)
		rows, err := db.Query(explainQuery, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type explainRow struct {
			ID           int    `json:"id"`
			SelectType   string `json:"select_type"`
			Table        string `json:"table"`
			Partitions   string `json:"partitions"`
			Type         string `json:"type"`
			PossibleKeys string `json:"possible_keys"`
			Key          string `json:"key"`
			KeyLen       int    `json:"key_len"`
			Ref          string `json:"ref"`
			Rows         int    `json:"rows"`
			Filtered     int    `json:"filtered"`
			Extra        string `json:"Extra"`
		}

		for rows.Next() {
			var row explainRow
			if err := rows.Scan(&row.ID, &row.SelectType, &row.Table, &row.Partitions, &row.Type, &row.PossibleKeys, &row.Key, &row.KeyLen, &row.Ref, &row.Rows, &row.Filtered, &row.Extra); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			explainResults = append(explainResults, ExplainResult{
				Table:        row.Table,
				PossibleKeys: strings.Split(row.PossibleKeys, ","),
				Key:          row.Key,
				Rows:         row.Rows,
				Filtered:     row.Filtered,
			})
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(explainResults)
}

func constructArgs(args []driver.Value, namedArgs []driver.NamedValue) []any {
	if args != nil {
		newArgs := make([]any, 0, len(args))
		for _, arg := range args {
			newArgs = append(newArgs, arg)
		}

		return newArgs
	}

	if namedArgs != nil {
		maxOrdinal := 0
		namedCount := 0
		othersCount := 0
		for _, namedArg := range namedArgs {
			switch {
			case namedArg.Name != "":
				namedCount++
			case namedArg.Ordinal > maxOrdinal:
				maxOrdinal = namedArg.Ordinal
			default:
				othersCount++
			}
		}

		newArgs := make([]any, maxOrdinal+namedCount+othersCount)
		namedIdx := maxOrdinal
		othersIdx := maxOrdinal + namedCount
		for _, namedArg := range namedArgs {
			switch {
			case namedArg.Name != "":
				newArgs[namedIdx] = namedArg.Value
				namedIdx++
			case namedArg.Ordinal > 0:
				newArgs[namedArg.Ordinal-1] = namedArg.Value
			default:
				newArgs[othersIdx] = namedArg.Value
				othersIdx++
			}
		}

		return newArgs
	}

	return nil
}

func tableListHandler(w http.ResponseWriter, r *http.Request) {
	driver := r.URL.Query().Get("driver")
	if driver == "" {
		http.Error(w, "driver is required", http.StatusBadRequest)
		return
	}

	db, ok := dbMap[driver]
	if !ok {
		http.Error(w, driver+" driver not found", http.StatusInternalServerError)
		return
	}

	tableMap := map[string]string{}
	switch driver {
	case "mysql":
		rows, err := db.Query("SHOW TABLES")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		tables := make([]string, 0, 10)
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			tables = append(tables, table)
		}

		for _, table := range tables {
			rows, err := db.Query(fmt.Sprintf("SHOW CREATE TABLE `%s`", table))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var createTable string
			if rows.Next() {
				var tmpTable string
				if err := rows.Scan(&tmpTable, &createTable); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			tableMap[table] = createTable
		}
	default:
		http.Error(w, "unsupported driver", http.StatusBadRequest)
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tableMap)
}

func Register(mux *http.ServeMux) {
	mux.Handle("GET /queries", http.HandlerFunc(queryListHandler))
	mux.Handle("GET /queries/{id}/explain", http.HandlerFunc(queryExplainHandler))
	mux.Handle("GET /tables", http.HandlerFunc(tableListHandler))
}
