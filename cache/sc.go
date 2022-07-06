package isucache

import (
	"context"
	"sync"
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

type Map[K comparable, V any] struct {
	m            map[K]V
	locker       sync.RWMutex
	loadMetrics  *prometheus.GaugeVec
	storeMetrics *prometheus.GaugeVec
}

func NewMap[K comparable, V any](name string) *Map[K, V] {
	var (
		loadMetrics  *prometheus.GaugeVec
		storeMetrics *prometheus.GaugeVec
	)
	if isutools.Enable {
		loadMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "load_count",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
		}, []string{"hit", "miss"})

		storeMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "store_count",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
		}, []string{"replace", "new", "remove"})
	}

	m := &Map[K, V]{
		m:            make(map[K]V),
		locker:       sync.RWMutex{},
		loadMetrics:  loadMetrics,
		storeMetrics: storeMetrics,
	}

	cacheMap[name] = m

	return m
}

func (m *Map[K, V]) Load(key K) (V, bool) {
	m.locker.RLock()

	v, ok := m.m[key]
	if ok {
		m.locker.RUnlock()

		if m.loadMetrics != nil {
			m.loadMetrics.WithLabelValues("hit").Inc()
		}

		return v, true
	}
	m.locker.RUnlock()

	if m.loadMetrics != nil {
		m.loadMetrics.WithLabelValues("miss").Inc()
	}

	return v, false
}

func (m *Map[K, V]) Store(key K, value V) {
	if m.loadMetrics != nil {
		m.locker.RLock()
		_, ok := m.m[key]
		m.locker.RUnlock()

		if ok {
			m.storeMetrics.WithLabelValues("replace").Inc()
		} else {
			m.storeMetrics.WithLabelValues("new").Inc()
		}
	}

	m.locker.Lock()
	m.m[key] = value
	m.locker.Unlock()
}

func (m *Map[K, V]) Forget(key K) {
	if m.storeMetrics != nil {
		m.locker.RLock()
		_, ok := m.m[key]
		m.locker.RUnlock()

		if ok {
			m.storeMetrics.WithLabelValues("remove").Inc()
		}
	}

	m.locker.Lock()
	delete(m.m, key)
	m.locker.Unlock()
}

func (m *Map[K, V]) Purge() {
	if m.storeMetrics != nil {
		m.storeMetrics.WithLabelValues("remove").Add(float64(len(m.m)))
	}

	m.locker.Lock()
	for k := range m.m {
		delete(m.m, k)
	}
	m.locker.Unlock()
}

func AllPurge() {
	for _, cache := range cacheMap {
		cache.Purge()
	}
}
