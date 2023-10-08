package isudb

import (
	"database/sql"
	"regexp"
	"sync"

	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("isusqlite3", wrapDriver(&sqlite3.SQLiteDriver{}, sqlite3SegmentBuilder{}))
}

type sqlite3SegmentBuilder struct{}

func (sqlite3SegmentBuilder) driver() string {
	return "sqlite3"
}

func (ssb sqlite3SegmentBuilder) parseDSN(dsn string) *measureSegment {
	return &measureSegment{
		driver:     ssb.driver(),
		addr:       dsn,
		normalizer: ssb.normalizer,
	}
}

var (
	sqliteReList = []struct {
		re *regexp.Regexp
		to string
	}{{
		re: regexp.MustCompile(`((?:\?(\d*)|[@:$][0-9A-Fa-f]+)\s*,\s*)+(?:\?(\d*)|[@:$][0-9A-Fa-f]+)`),
		to: "..., ?",
	}, {
		re: regexp.MustCompile(`(\(\.\.\., \?\)\s*,\s*)+`),
		to: "..., ",
	}}
	sqlite3NormalizeCacheLocker = &sync.RWMutex{}
	sqlite3NormalizeCache       = make(map[string]string, 50)
)

func (sqlite3SegmentBuilder) normalizer(query string) string {
	var (
		normalizedQuery string
		ok              bool
	)
	func() {
		sqlite3NormalizeCacheLocker.RLock()
		defer sqlite3NormalizeCacheLocker.RUnlock()
		normalizedQuery, ok = sqlite3NormalizeCache[query]
	}()
	if ok {
		return normalizedQuery
	}

	normalizedQuery = query
	for _, re := range sqliteReList {
		normalizedQuery = re.re.ReplaceAllString(normalizedQuery, re.to)
	}

	func() {
		sqlite3NormalizeCacheLocker.Lock()
		defer sqlite3NormalizeCacheLocker.Unlock()
		sqlite3NormalizeCache[query] = normalizedQuery
	}()

	return normalizedQuery
}
