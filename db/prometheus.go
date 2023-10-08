package isudb

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "db"
)

var queryDurBuckets = prometheus.DefBuckets

var (
	queryCountVec = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "query_count",
	}, []string{"driver", "addr", "query"})
	queryDurHistogramVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "query_duration_seconds",
		Buckets:   queryDurBuckets,
	}, []string{"driver", "addr", "query"})
)
