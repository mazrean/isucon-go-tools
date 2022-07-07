package isucache

import (
	"context"
	"sync"
	"sync/atomic"
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
		}, []string{"status"})

		storeMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "store_count",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
		}, []string{"status"})
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

type AtomicMap[K comparable, V *T, T any] struct {
	m            map[K]*atomic.Pointer[T]
	locker       sync.RWMutex
	loadMetrics  *prometheus.GaugeVec
	storeMetrics *prometheus.GaugeVec
}

func NewAtomicMap[K comparable, V *T, T any](name string) *AtomicMap[K, V, T] {
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
		}, []string{"status"})

		storeMetrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "store_count",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
		}, []string{"status"})
	}

	m := &AtomicMap[K, V, T]{
		m:            make(map[K]*atomic.Pointer[T]),
		locker:       sync.RWMutex{},
		loadMetrics:  loadMetrics,
		storeMetrics: storeMetrics,
	}

	cacheMap[name] = m

	return m
}

func (m *AtomicMap[K, V, T]) Load(key K) (V, bool) {
	m.locker.RLock()
	v, ok := m.m[key]
	if ok {
		val := v.Load()
		m.locker.RUnlock()

		if m.loadMetrics != nil {
			m.loadMetrics.WithLabelValues("hit").Inc()
		}

		return val, true
	}
	m.locker.RUnlock()

	if m.loadMetrics != nil {
		m.loadMetrics.WithLabelValues("miss").Inc()
	}

	return nil, false
}

func (m *AtomicMap[K, V, T]) Store(key K, value V) {
	m.locker.RLock()
	v, ok := m.m[key]
	if ok {
		v.Store((*T)(value))
		m.locker.RUnlock()

		if m.storeMetrics != nil {
			m.storeMetrics.WithLabelValues("replace").Inc()
		}

		return
	}
	m.locker.RUnlock()

	v = &atomic.Pointer[T]{}
	v.Store((*T)(value))
	m.locker.Lock()
	m.m[key] = v
	m.locker.Unlock()

	if m.loadMetrics != nil {
		m.storeMetrics.WithLabelValues("new").Inc()
	}
}

func (m *AtomicMap[K, V, T]) Forget(key K) {
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

func (m *AtomicMap[K, V, T]) Purge() {
	if m.storeMetrics != nil {
		m.storeMetrics.WithLabelValues("remove").Set(0)
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
