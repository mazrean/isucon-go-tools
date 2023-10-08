package isudb

import (
	"database/sql"
	"regexp"
	"strings"
	"sync"

	"github.com/lib/pq"
)

func init() {
	sql.Register("isupostgres", wrapDriver(pq.Driver{}, postgresSegmentBuilder{}))
}

type postgresSegmentBuilder struct{}

func (postgresSegmentBuilder) driver() string {
	return "postgres"
}

func (psb postgresSegmentBuilder) parseDSN(dsn string) *measureSegment {
	strConfig, err := pq.ParseURL(dsn)
	if err != nil {
		strConfig = dsn
	}

	splitedConfigs := strings.Split(strConfig, " ")
	host := "localhost"
	port := "5432"
	for _, config := range splitedConfigs {
		switch {
		case strings.HasPrefix(config, "host="):
			host = strings.TrimPrefix(config, "host=")
		case strings.HasPrefix(config, "port="):
			port = strings.TrimPrefix(config, "port=")
		}
	}

	return &measureSegment{
		driver:     psb.driver(),
		addr:       host + ":" + port,
		normalizer: psb.normalizer,
	}
}

var (
	postgresReList = []struct {
		re *regexp.Regexp
		to string
	}{{
		re: regexp.MustCompile(`(\$(\d*)\s*,\s*)+\$(\d*)`),
		to: "..., ?",
	}, {
		re: regexp.MustCompile(`(\(..., \?\)\s*,\s*)+`),
		to: "..., ",
	}}
	postgresNormalizeCacheLocker = &sync.RWMutex{}
	postgresNormalizeCache       = make(map[string]string, 50)
)

func (postgresSegmentBuilder) normalizer(query string) string {
	var (
		normalizedQuery string
		ok              bool
	)
	func() {
		postgresNormalizeCacheLocker.RLock()
		defer postgresNormalizeCacheLocker.RUnlock()
		normalizedQuery, ok = postgresNormalizeCache[query]
	}()
	if ok {
		return normalizedQuery
	}

	normalizedQuery = query
	for _, re := range postgresReList {
		normalizedQuery = re.re.ReplaceAllString(normalizedQuery, re.to)
	}

	func() {
		postgresNormalizeCacheLocker.Lock()
		defer postgresNormalizeCacheLocker.Unlock()
		postgresNormalizeCache[query] = normalizedQuery
	}()

	return normalizedQuery
}
