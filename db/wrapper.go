package isudb

import (
	"context"
	"database/sql/driver"
	"time"

	isudbgen "github.com/mazrean/isucon-go-tools/db/internal/generate"
)

type wrappedDriver struct {
	driver.Driver
	segmentBuilder SegmentBuilder
}

func wrapDriver(d driver.Driver, segmentBuilder SegmentBuilder) driver.Driver {
	return isudbgen.LimitOptionalDriver(d, func(d driver.Driver) isudbgen.LimitOptionalDriverWrappedType {
		return &wrappedDriver{
			Driver:         d,
			segmentBuilder: segmentBuilder,
		}
	})
}

func (wd *wrappedDriver) Open(name string) (driver.Conn, error) {
	conn, err := wd.Driver.Open(name)
	if err != nil {
		return nil, err
	}

	return wrapConn(conn, wd.segmentBuilder.ParseDSN(name)), nil
}

func (wd *wrappedDriver) OpenConnector(name string) (driver.Connector, error) {
	conn, err := wd.Driver.(driver.DriverContext).OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return wrapConnector(conn, wd, wd.segmentBuilder.ParseDSN(name)), nil
}

type wrappedConn struct {
	driver.Conn
	segment *measureSegment
}

func wrapConn(conn driver.Conn, segment *measureSegment) driver.Conn {
	return isudbgen.LimitOptionalConn(conn, func(c driver.Conn) isudbgen.LimitOptionalConnWrappedType {
		return &wrappedConn{
			Conn:    conn,
			segment: segment,
		}
	})
}

func (wc *wrappedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := wc.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}

	return wrapStmt(stmt, wc.segment, query), nil
}

func (wc *wrappedConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	stmt, err := wc.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}

	return wrapStmt(stmt, wc.segment, query), nil
}

func (wc *wrappedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return wc.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func (wc *wrappedConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return measureQuery(wc.segment, query, func() (driver.Result, error) {
		return wc.Conn.(driver.Execer).Exec(query, args)
	})
}

func (wc *wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return measureQuery(wc.segment, query, func() (driver.Result, error) {
		return wc.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	})
}

func (wc *wrappedConn) CheckNamedValue(v *driver.NamedValue) error {
	return wc.Conn.(driver.NamedValueChecker).CheckNamedValue(v)
}

func (wc *wrappedConn) Ping(ctx context.Context) error {
	return wc.Conn.(driver.Pinger).Ping(ctx)
}

func (wc *wrappedConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return measureQuery(wc.segment, query, func() (driver.Rows, error) {
		return wc.Conn.(driver.Queryer).Query(query, args)
	})
}

func (wc *wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return measureQuery(wc.segment, query, func() (driver.Rows, error) {
		return wc.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	})
}

func (wc *wrappedConn) ResetSession(ctx context.Context) error {
	return wc.Conn.(driver.SessionResetter).ResetSession(ctx)
}

type wrappedConnector struct {
	driver.Connector
	driver  driver.Driver
	segment *measureSegment
}

func wrapConnector(connector driver.Connector, driver driver.Driver, segment *measureSegment) driver.Connector {
	return &wrappedConnector{
		Connector: connector,
		driver:    driver,
		segment:   segment,
	}
}

func (wc *wrappedConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := wc.Connector.Connect(ctx)
	if nil != err {
		return nil, err
	}

	return wrapConn(conn, wc.segment), nil
}

func (wc *wrappedConnector) Driver() driver.Driver {
	return wc.driver
}

type wrappedStmt struct {
	driver.Stmt
	segment *measureSegment
	query   string
}

func wrapStmt(stmt driver.Stmt, segment *measureSegment, query string) driver.Stmt {
	return isudbgen.LimitOptionalStmt(stmt, func(s driver.Stmt) isudbgen.LimitOptionalStmtWrappedType {
		return &wrappedStmt{
			Stmt:    s,
			segment: segment,
			query:   query,
		}
	})
}

func (ws *wrappedStmt) ColumnConverter(idx int) driver.ValueConverter {
	return ws.Stmt.(driver.ColumnConverter).ColumnConverter(idx)
}

func (ws *wrappedStmt) CheckNamedValue(v *driver.NamedValue) error {
	return ws.Stmt.(driver.NamedValueChecker).CheckNamedValue(v)
}

func (ws *wrappedStmt) Exec(args []driver.Value) (driver.Result, error) {
	return measureQuery(ws.segment, ws.query, func() (driver.Result, error) {
		return ws.Stmt.Exec(args)
	})
}

func (ws *wrappedStmt) Query(args []driver.Value) (driver.Rows, error) {
	return measureQuery(ws.segment, ws.query, func() (driver.Rows, error) {
		return ws.Stmt.Query(args)
	})
}

func (ws *wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return measureQuery(ws.segment, ws.query, func() (driver.Result, error) {
		return ws.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
	})
}

func (ws *wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return measureQuery(ws.segment, ws.query, func() (driver.Rows, error) {
		return ws.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
	})
}

type measureSegment struct {
	driver string
	addr   string
}

type SegmentBuilder interface {
	Driver() string
	ParseDSN(dsn string) *measureSegment
}

func (m *measureSegment) setQueryResult(query string, queryDur float64) {
	if enableQueryTrace {
		queryCountVec.WithLabelValues(m.driver, m.addr, query).Inc()
		queryDurHistogramVec.WithLabelValues(m.driver, m.addr, query).Observe(queryDur)
	}
}

func measureQuery[T any](segment *measureSegment, query string, f func() (T, error)) (T, error) {
	start := time.Now()
	result, err := f()
	queryDur := float64(time.Since(start)) / float64(time.Second)

	segment.setQueryResult(query, queryDur)

	return result, err
}
