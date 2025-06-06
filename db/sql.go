package isudb

import (
	"database/sql"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var connectionID = &atomic.Uint64{}

func init() {
	connectionID.Store(0)
}

type DB = interface {
	Ping() error
	Close() error
	Query(query string, args ...any) (*sql.Rows, error)
	SetConnMaxIdleTime(d time.Duration)
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}

var dbMap = map[string]DB{}

func DBMetricsSetup[T DB](fn func(string, string) (T, error)) func(string, string) (T, error) {
	return func(driverName string, dataSourceName string) (T, error) {
		openDriverName := driverName
		var addr string
		switch driverName {
		case "mysql":
			if fixInterpolateParams {
				config, err := mysql.ParseDSN(dataSourceName)
				if err != nil {
					slog.Error("failed to parse DSN",
						slog.String("driver", driverName),
						slog.String("dsn", dataSourceName),
						slog.String("error", err.Error()),
					)
					goto CONNECT
				}

				if !config.InterpolateParams {
					config.InterpolateParams = true
					dataSourceName = config.FormatDSN()
				}

				addr = config.Addr
			}

			if config.Enable {
				openDriverName = "isumysql"
			}
		case "sqlite3":
			if config.Enable {
				openDriverName = "isusqlite3"
			}
		case "postgres":
			if config.Enable {
				openDriverName = "isupostgres"
			}
		}

	CONNECT:
		var (
			db  T
			err error
		)
		if enableRetry {
			var (
				first = true
				err   error
			)
			for first || err != nil {
				first = false
				db, err = fn(openDriverName, dataSourceName)
				if err != nil {
					return db, err
				}

				err = db.Ping()
				if err != nil {
					db.Close()
				}
			}
		} else {
			db, err = fn(openDriverName, dataSourceName)
			if err != nil {
				return db, err
			}

			err = db.Ping()
			if err != nil {
				db.Close()
				return db, err
			}
		}

		db.SetMaxIdleConns(1024)
		db.SetConnMaxLifetime(0)
		db.SetConnMaxIdleTime(0)

		if config.Enable {
			connID := connectionID.Add(1)
			strConnID := strconv.FormatUint(connID, 10)

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_open_connections",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"status":        "idle",
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().Idle)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"status":        "open",
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"status":        "in_use",
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().InUse)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_count",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().WaitCount)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_duration",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().WaitDuration)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_closed",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().MaxOpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_lifetime_closed",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().MaxLifetimeClosed)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_time_closed",
				ConstLabels: map[string]string{
					"driver":        driverName,
					"addr":          addr,
					"connection_id": strConnID,
				},
			}, func() float64 {
				return float64(db.Stats().MaxIdleTimeClosed)
			})

			dbMap[driverName] = db
		}

		return db, err
	}
}
