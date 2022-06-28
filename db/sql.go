package db

import (
	"database/sql"

	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "db"
)

func InitDBMetrics(db interface {
	Stats() sql.DBStats
}) {
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
}
