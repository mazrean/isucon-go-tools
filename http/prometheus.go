package isuhttp

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "api"
)

const (
	KB float64 = 1 << (10 * (iota + 1))
	MB
	GB
	TB
)

var reqDurBuckets = prometheus.DefBuckets

var reqSzBuckets = []float64{1.0 * KB, 2.0 * KB, 5.0 * KB, 10.0 * KB, 100 * KB, 500 * KB, 1.0 * MB, 2.5 * MB, 5.0 * MB, 10.0 * MB}

var resSzBuckets = []float64{1.0 * KB, 2.0 * KB, 5.0 * KB, 10.0 * KB, 100 * KB, 500 * KB, 1.0 * MB, 2.5 * MB, 5.0 * MB, 10.0 * MB}

var reqCounterVec = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: prometheusNamespace,
	Subsystem: prometheusSubsystem,
	Name:      "request_total",
}, []string{"code", "method", "host", "url"})

var reqDurHistogramVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: prometheusNamespace,
	Subsystem: prometheusSubsystem,
	Name:      "request_duration_seconds",
	Buckets:   reqDurBuckets,
}, []string{"code", "method", "url"})

var resSizeHistogramVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: prometheusNamespace,
	Subsystem: prometheusSubsystem,
	Name:      "response_size_bytes",
	Buckets:   resSzBuckets,
}, []string{"code", "method", "url"})

var reqSizeHistogramVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: prometheusNamespace,
	Subsystem: prometheusSubsystem,
	Name:      "request_size_bytes",
	Buckets:   reqSzBuckets,
}, []string{"code", "method", "url"})

// reqSize 大まかなリクエストサイズ
func reqSize(req *http.Request) float64 {
	size := 0.0
	if req.URL != nil {
		size += float64(len(req.URL.RawPath))
	}

	size += float64(len(req.Method))
	size += float64(len(req.Proto))
	for name, values := range req.Header {
		size += float64(len(name))
		for _, value := range values {
			size += float64(len(value))
		}
	}
	size += float64(len(req.Host))

	if req.ContentLength > 0 {
		size += float64(req.ContentLength)
	}

	return size
}
