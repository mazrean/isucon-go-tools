package isudb

import (
	"database/sql"
	"regexp"
	"sync"

	"github.com/go-sql-driver/mysql"
)

func init() {
	sql.Register("isumysql", wrapDriver(mysql.MySQLDriver{}, mysqlSegmentBuilder{}))
}

type mysqlSegmentBuilder struct{}

func (mysqlSegmentBuilder) driver() string {
	return "mysql"
}

func (msb mysqlSegmentBuilder) parseDSN(dsn string) *measureSegment {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic(err)
	}

	return &measureSegment{
		driver:     msb.driver(),
		addr:       cfg.Addr,
		normalizer: msb.normalizer,
	}
}

var (
	mysqlReList = []struct {
		re *regexp.Regexp
		to string
	}{{
		re: regexp.MustCompile(`(\?\s*,\s*)+`),
		to: "..., ",
	}, {
		re: regexp.MustCompile(`(\(\.\.\., \?\)\s*,\s*)+`),
		to: "..., ",
	}}
	mysqlNormalizeCacheLocker = &sync.RWMutex{}
	mysqlNormalizeCache       = make(map[string]string, 50)
)

func (mysqlSegmentBuilder) normalizer(query string) string {
	var (
		normalizedQuery string
		ok              bool
	)
	func() {
		mysqlNormalizeCacheLocker.RLock()
		defer mysqlNormalizeCacheLocker.RUnlock()
		normalizedQuery, ok = mysqlNormalizeCache[query]
	}()
	if ok {
		return normalizedQuery
	}

	normalizedQuery = query
	for _, re := range mysqlReList {
		normalizedQuery = re.re.ReplaceAllString(normalizedQuery, re.to)
	}

	func() {
		mysqlNormalizeCacheLocker.Lock()
		defer mysqlNormalizeCacheLocker.Unlock()
		mysqlNormalizeCache[query] = normalizedQuery
	}()

	return normalizedQuery
}
