package isudb

import (
	"database/sql"

	"github.com/mattn/go-sqlite3"
)

func init() {
	sql.Register("isusqlite3", wrapDriver(&sqlite3.SQLiteDriver{}, sqlite3SegmentBuilder{}))
}

type sqlite3SegmentBuilder struct{}

func (sqlite3SegmentBuilder) Driver() string {
	return "sqlite3"
}

func (ssb sqlite3SegmentBuilder) ParseDSN(dsn string) *measureSegment {
	return &measureSegment{
		driver: ssb.Driver(),
		addr:   dsn,
	}
}
