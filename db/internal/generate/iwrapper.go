package isudbgen

//go:generate go run github.com/mazrean/iwrapper -src=$GOFILE -dst=iwrapper_gen.go

import "database/sql/driver"

//iwrapper:target
type Conn interface {
	//iwrapper:require
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	//nolint:staticcheck
	driver.Execer
	driver.ExecerContext
	driver.NamedValueChecker
	driver.Pinger
	//nolint:staticcheck
	driver.Queryer
	driver.QueryerContext
}

//iwrapper:target
type Driver interface {
	//iwrapper:require
	driver.Driver
	driver.DriverContext
}

//iwrapper:target
type Stmt interface {
	//iwrapper:require
	driver.Stmt
	//nolint:staticcheck
	driver.ColumnConverter
	driver.NamedValueChecker
	driver.StmtExecContext
	driver.StmtQueryContext
}
