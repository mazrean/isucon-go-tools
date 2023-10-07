package isudb

import (
	"database/sql"

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
		driver: msb.driver(),
		addr:   cfg.Addr,
	}
}
