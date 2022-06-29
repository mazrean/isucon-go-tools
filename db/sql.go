package isudb

import (
	"database/sql"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "db"
)

var (
	enableRetry          = false
	fixInterpolateParams = true
)

func SetRetry(enable bool) {
	enableRetry = enable
}

func SetFixInterpolateParams(enable bool) {
	fixInterpolateParams = enable
}

func DBMetricsSetup[T interface {
	Ping() error
	Close() error
	SetConnMaxIdleTime(d time.Duration)
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}](fn func(string, string) (T, error)) func(string, string) (T, error) {
	return func(driverName string, dataSourceName string) (T, error) {
		if fixInterpolateParams && driverName == "mysql" {
			config, err := mysql.ParseDSN(dataSourceName)
			if err != nil {
				log.Printf("failed to parse dsn: %v\n", err)
				goto CONNECT
			}

			if !config.InterpolateParams {
				config.InterpolateParams = true
				dataSourceName = config.FormatDSN()
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
				db, err = fn(driverName, dataSourceName)
				if err != nil {
					return db, err
				}

				err = db.Ping()
				if err != nil {
					db.Close()
				}
			}
		} else {
			db, err = fn(driverName, dataSourceName)
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

		if isutools.Enable {
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_open_connections",
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "idle",
				},
			}, func() float64 {
				return float64(db.Stats().Idle)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "open",
				},
			}, func() float64 {
				return float64(db.Stats().OpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "connection_pool",
				ConstLabels: map[string]string{
					"status": "in_use",
				},
			}, func() float64 {
				return float64(db.Stats().InUse)
			})

			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_count",
			}, func() float64 {
				return float64(db.Stats().WaitCount)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "wait_duration",
			}, func() float64 {
				return float64(db.Stats().WaitDuration)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_closed",
			}, func() float64 {
				return float64(db.Stats().MaxOpenConnections)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_lifetime_closed",
			}, func() float64 {
				return float64(db.Stats().MaxLifetimeClosed)
			})
			promauto.NewGaugeFunc(prometheus.GaugeOpts{
				Namespace: prometheusNamespace,
				Subsystem: prometheusSubsystem,
				Name:      "max_idle_time_closed",
			}, func() float64 {
				return float64(db.Stats().MaxIdleTimeClosed)
			})
		}

		return db, err
	}
}
