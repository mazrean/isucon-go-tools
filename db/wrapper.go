package isudb

import (
	"context"
	"database/sql/driver"
	"time"

	isudbgen "github.com/mazrean/isucon-go-tools/v2/db/internal/generate"
)

type wrappedDriver struct {
	driver.Driver
	segmentBuilder segmentBuilder
}

func wrapDriver(d driver.Driver, segBuilder segmentBuilder) driver.Driver {
	return isudbgen.DriverWrapper(d, func(d driver.Driver) isudbgen.Driver {
		return &wrappedDriver{
			Driver:         d,
			segmentBuilder: segBuilder,
		}
	})
}

func (wd *wrappedDriver) Open(name string) (driver.Conn, error) {
	conn, err := wd.Driver.Open(name)
	if err != nil {
		return nil, err
	}

	return wrapConn(conn, wd.segmentBuilder.parseDSN(name)), nil
}

func (wd *wrappedDriver) OpenConnector(name string) (driver.Connector, error) {
	conn, err := wd.Driver.(driver.DriverContext).OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return wrapConnector(conn, wd, wd.segmentBuilder.parseDSN(name)), nil
}

type wrappedConn struct {
	driver.Conn
	segment *measureSegment
}

func wrapConn(conn driver.Conn, segment *measureSegment) driver.Conn {
	return isudbgen.ConnWrapper(conn, func(c driver.Conn) isudbgen.Conn {
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
	return measureQuery(wc.segment, query, args, nil, func() (driver.Result, error) {
		//nolint:staticcheck
		return wc.Conn.(driver.Execer).Exec(query, args)
	})
}

func (wc *wrappedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return measureQuery(wc.segment, query, nil, args, func() (driver.Result, error) {
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
	return measureQuery(wc.segment, query, args, nil, func() (driver.Rows, error) {
		//nolint:staticcheck
		return wc.Conn.(driver.Queryer).Query(query, args)
	})
}

func (wc *wrappedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return measureQuery(wc.segment, query, nil, args, func() (driver.Rows, error) {
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
	return isudbgen.StmtWrapper(stmt, func(s driver.Stmt) isudbgen.Stmt {
		return &wrappedStmt{
			Stmt:    s,
			segment: segment,
			query:   query,
		}
	})
}

func (ws *wrappedStmt) ColumnConverter(idx int) driver.ValueConverter {
	//nolint:staticcheck
	return ws.Stmt.(driver.ColumnConverter).ColumnConverter(idx)
}

func (ws *wrappedStmt) CheckNamedValue(v *driver.NamedValue) error {
	return ws.Stmt.(driver.NamedValueChecker).CheckNamedValue(v)
}

func (ws *wrappedStmt) Exec(args []driver.Value) (driver.Result, error) {
	return measureQuery(ws.segment, ws.query, args, nil, func() (driver.Result, error) {
		//nolint:staticcheck
		return ws.Stmt.Exec(args)
	})
}

func (ws *wrappedStmt) Query(args []driver.Value) (driver.Rows, error) {
	return measureQuery(ws.segment, ws.query, args, nil, func() (driver.Rows, error) {
		//nolint:staticcheck
		return ws.Stmt.Query(args)
	})
}

func (ws *wrappedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	return measureQuery(ws.segment, ws.query, nil, args, func() (driver.Result, error) {
		return ws.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
	})
}

func (ws *wrappedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	return measureQuery(ws.segment, ws.query, nil, args, func() (driver.Rows, error) {
		return ws.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
	})
}

type measureSegment struct {
	driver     string
	addr       string
	normalizer func(string) string
}

type segmentBuilder interface {
	driver() string
	parseDSN(dsn string) *measureSegment
}

func (m *measureSegment) setQueryResult(query string, args []driver.Value, namedArgs []driver.NamedValue, queryDur float64) {
	if enableQueryTrace {
		normalizedQuery := m.normalizeQuery(query)

		queryCountVec.WithLabelValues(m.driver, m.addr, normalizedQuery).Inc()
		queryDurHistogramVec.WithLabelValues(m.driver, m.addr, normalizedQuery).Observe(queryDur)
		queryExecHook(m.driver, normalizedQuery, query, args, namedArgs, queryDur)
	}
}

func measureQuery[T any](segment *measureSegment, query string, args []driver.Value, namedArgs []driver.NamedValue, f func() (T, error)) (T, error) {
	start := time.Now()
	result, err := f()
	queryDur := float64(time.Since(start)) / float64(time.Second)

	segment.setQueryResult(query, args, namedArgs, queryDur)

	return result, err
}

func (m *measureSegment) normalizeQuery(query string) string {
	if m.normalizer != nil {
		return m.normalizer(query)
	}

	return query
}
