package isudb

import (
	"database/sql"

	"github.com/go-sql-driver/mysql"
)

func init() {
	sql.Register("isumysql", wrapDriver(mysql.MySQLDriver{}, mysqlSegmentBuilder{}))
}

type mysqlSegmentBuilder struct{}

func (mysqlSegmentBuilder) Driver() string {
	return "mysql"
}

func (msb mysqlSegmentBuilder) ParseDSN(dsn string) *measureSegment {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		panic(err)
	}

	return &measureSegment{
		driver: msb.Driver(),
		addr:   cfg.Addr,
	}
}
