package isudb

import (
	"database/sql"

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
		driver: ssb.driver(),
		addr:   dsn,
	}
}
