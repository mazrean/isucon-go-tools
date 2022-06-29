package isucache

import (
	"context"
	"time"

	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/motoki317/sc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "cache"
)

var (
	cacheMap = make(map[string]interface {
		Purge()
	}, 0)
)

func New[K comparable, V any](name string, replaceFn func(ctx context.Context, key K) (V, error), freshFor, ttl time.Duration, options ...sc.CacheOption) (*sc.Cache[K, V], error) {
	cache, err := sc.New(replaceFn, freshFor, ttl, options...)
	if err != nil {
		return cache, err
	}

	cacheMap[name] = cache

	if isutools.Enable {
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "hit_count",
			ConstLabels: prometheus.Labels{
				"name": name,
				"stat": "hit",
			},
		}, func() float64 {
			return float64(cache.Stats().Hits)
		})
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "hit_count",
			ConstLabels: prometheus.Labels{
				"name": name,
				"stat": "grace_hit",
			},
		}, func() float64 {
			return float64(cache.Stats().GraceHits)
		})
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "hit_count",
			ConstLabels: prometheus.Labels{
				"name": name,
				"stat": "miss",
			},
		}, func() float64 {
			return float64(cache.Stats().Misses)
		})
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "hit_count",
			ConstLabels: prometheus.Labels{
				"name": name,
				"stat": "replace",
			},
		}, func() float64 {
			return float64(cache.Stats().Replacements)
		})
	}

	return cache, nil
}

func AllPurge() {
	for _, cache := range cacheMap {
		cache.Purge()
	}
}
