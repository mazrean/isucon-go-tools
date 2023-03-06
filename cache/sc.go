package isucache

import (
	"context"
	"encoding/gob"
	"fmt"
	"io"
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
	defer m.locker.RUnlock()

	v, ok := m.m[key]
	if ok {
		if m.loadMetrics != nil {
			m.loadMetrics.WithLabelValues("hit").Inc()
		}

		return v, true
	}

	if m.loadMetrics != nil {
		m.loadMetrics.WithLabelValues("miss").Inc()
	}

	return v, false
}

func (m *Map[K, V]) LoadOrStore(key K, value V) (V, bool) {
	m.locker.Lock()
	defer m.locker.Unlock()

	v, ok := m.m[key]
	if ok {
		return v, true
	}

	m.m[key] = value

	return value, false
}

func (m *Map[K, V]) Store(key K, value V) {
	if m.loadMetrics != nil {
		m.locker.RLock()
		defer m.locker.RUnlock()

		_, ok := m.m[key]

		if ok {
			m.storeMetrics.WithLabelValues("replace").Inc()
		} else {
			m.storeMetrics.WithLabelValues("new").Inc()
		}
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	m.m[key] = value
}

func (m *Map[K, V]) Len() int {
	m.locker.RLock()
	defer m.locker.RUnlock()

	l := len(m.m)
	return l
}

func (m *Map[K, V]) Update(key K, f func(V) (V, bool)) {
	m.locker.Lock()
	defer m.locker.Unlock()

	v, ok := f(m.m[key])
	if ok {
		m.m[key] = v
	}
}

func (m *Map[K, V]) RangeUpdate(f func(K, V) (V, bool)) {
	m.locker.Lock()
	defer m.locker.Unlock()

	for k, v := range m.m {
		v, ok := f(k, v)
		if ok {
			m.m[k] = v
		}
	}
}

func (m *Map[K, V]) Forget(key K) {
	if m.storeMetrics != nil {
		m.locker.RLock()
		defer m.locker.RUnlock()
		_, ok := m.m[key]

		if ok {
			m.storeMetrics.WithLabelValues("remove").Inc()
		}
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	delete(m.m, key)
}

func (m *Map[K, V]) Range(f func(K, V) bool) {
	m.locker.RLock()
	defer m.locker.RUnlock()

	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
}

func (m *Map[K, V]) Purge() {
	if m.storeMetrics != nil {
		m.storeMetrics.WithLabelValues("remove").Add(float64(len(m.m)))
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	for k := range m.m {
		delete(m.m, k)
	}
}

func (m *Map[K, V]) WriteToGob(w io.Writer) error {
	m.locker.RLock()
	defer m.locker.RUnlock()

	err := gob.NewEncoder(w).Encode(m.m)
	if err != nil {
		return fmt.Errorf("failed to write to gob: %w", err)
	}

	return nil
}

func (m *Map[K, V]) LoadFromGob(r io.Reader) error {
	m.locker.Lock()
	defer m.locker.Unlock()

	for k := range m.m {
		delete(m.m, k)
	}

	err := gob.NewDecoder(r).Decode(&m.m)
	if err != nil {
		return fmt.Errorf("failed to load from gob: %w", err)
	}

	return nil
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
	defer m.locker.RUnlock()

	v, ok := m.m[key]
	if ok {
		val := v.Load()

		if m.loadMetrics != nil {
			m.loadMetrics.WithLabelValues("hit").Inc()
		}

		return val, true
	}

	if m.loadMetrics != nil {
		m.loadMetrics.WithLabelValues("miss").Inc()
	}

	return nil, false
}

func (m *AtomicMap[K, V, T]) LoadOrStore(key K, value V) (V, bool) {
	m.locker.Lock()
	defer m.locker.Unlock()
	v, ok := m.m[key]
	if ok {
		val := v.Load()

		return val, true
	}

	v = &atomic.Pointer[T]{}
	v.Store((*T)(value))
	m.m[key] = v

	return value, false
}

func (m *AtomicMap[K, V, T]) Store(key K, value V) {
	v, ok := func() (*atomic.Pointer[T], bool) {
		m.locker.RLock()
		defer m.locker.RUnlock()

		v, ok := m.m[key]

		return v, ok
	}()
	if ok {
		v.Store((*T)(value))

		if m.storeMetrics != nil {
			m.storeMetrics.WithLabelValues("replace").Inc()
		}

		return
	}

	v = &atomic.Pointer[T]{}
	v.Store((*T)(value))

	m.locker.Lock()
	defer m.locker.Unlock()

	m.m[key] = v

	if m.loadMetrics != nil {
		m.storeMetrics.WithLabelValues("new").Inc()
	}
}

func (m *AtomicMap[K, V, T]) Len() int {
	m.locker.RLock()
	defer m.locker.RUnlock()

	return len(m.m)
}

func (m *AtomicMap[K, V, T]) Update(key K, f func(V) (V, bool)) {
	m.locker.Lock()
	defer m.locker.Unlock()
	v, ok := f(m.m[key].Load())
	if ok {
		m.m[key].Store((*T)(v))
	}
}

func (m *AtomicMap[K, V, T]) RangeUpdate(f func(K, V) (V, bool)) {
	m.locker.Lock()
	defer m.locker.Unlock()

	for k, vp := range m.m {
		v, ok := f(k, vp.Load())
		if ok {
			vp.Store((*T)(v))
		}
	}
}

func (m *AtomicMap[K, V, T]) Forget(key K) {
	if m.storeMetrics != nil {
		func() {
			m.locker.RLock()
			defer m.locker.RUnlock()
			_, ok := m.m[key]

			if ok {
				m.storeMetrics.WithLabelValues("remove").Inc()
			}
		}()
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	delete(m.m, key)
}

func (m *AtomicMap[K, V, T]) Range(f func(K, V) bool) {
	m.locker.RLock()
	defer m.locker.RUnlock()
	for k, vp := range m.m {
		v := vp.Load()
		if !f(k, v) {
			break
		}
	}
}

func (m *AtomicMap[K, V, T]) Purge() {
	if m.storeMetrics != nil {
		m.storeMetrics.WithLabelValues("remove").Set(0)
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	for k := range m.m {
		delete(m.m, k)
	}
}

func (m *AtomicMap[K, V, T]) WriteToGob(w io.Writer) error {
	m.locker.RLock()
	defer m.locker.RUnlock()

	gobMap := make(map[K]V, len(m.m))
	for k, vp := range m.m {
		v := vp.Load()
		gobMap[k] = v
	}

	err := gob.NewEncoder(w).Encode(gobMap)
	if err != nil {
		return fmt.Errorf("failed to write to gob: %w", err)
	}

	return nil
}

func (m *AtomicMap[K, V, T]) LoadFromGob(r io.Reader) error {
	gobMap := make(map[K]V, len(m.m))
	err := gob.NewDecoder(r).Decode(&gobMap)
	if err != nil {
		return fmt.Errorf("failed to load from gob: %w", err)
	}

	m.locker.Lock()
	defer m.locker.Unlock()

	for k := range m.m {
		delete(m.m, k)
	}
	for k, v := range gobMap {
		m.m[k] = &atomic.Pointer[T]{}
		m.m[k].Store((*T)(v))
	}

	return nil
}

type Slice[T any] struct {
	s             []T
	locker        sync.RWMutex
	indexMetrics  prometheus.Histogram
	lengthMetrics prometheus.Gauge
}

func NewSlice[T any](name string, init []T, size int) *Slice[T] {
	var (
		indexMetrics  prometheus.Histogram
		lengthMetrics prometheus.Gauge
	)
	if isutools.Enable {
		indexMetrics = promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "index_access",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
			Buckets: prometheus.LinearBuckets(0, float64(size)/20, 20),
		})

		lengthMetrics = promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "length",
			ConstLabels: prometheus.Labels{
				"name": name,
			},
		})
	}

	m := &Slice[T]{
		s:             init,
		locker:        sync.RWMutex{},
		indexMetrics:  indexMetrics,
		lengthMetrics: lengthMetrics,
	}

	cacheMap[name] = m

	return m
}

func (s *Slice[T]) Set(index int, value T) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.s[index] = value

	if s.lengthMetrics != nil {
		s.lengthMetrics.Set(float64(len(s.s)))
	}
}

func (s *Slice[T]) Slice(start, end int, f func([]T)) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	f(s.s[start:end])
}

func (s *Slice[T]) Get(i int) (T, bool) {
	if s.indexMetrics != nil {
		s.indexMetrics.Observe(float64(i))
	}

	s.locker.RLock()
	defer s.locker.RUnlock()
	if i >= len(s.s) {
		var v T
		return v, false
	}

	return s.s[i], true
}

func (s *Slice[T]) Edit(f func([]T) []T) {
	s.locker.Lock()
	defer s.locker.Unlock()
	s.s = f(s.s)
}

func (s *Slice[T]) Append(values ...T) {
	s.locker.Lock()
	defer s.locker.Unlock()

	s.s = append(s.s, values...)

	if s.lengthMetrics != nil {
		s.lengthMetrics.Set(float64(len(s.s)))
	}
}

func (s *Slice[T]) Len() int {
	return len(s.s)
}

func (s *Slice[T]) Range(f func(int, T) bool) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	for i, v := range s.s {
		if s.indexMetrics != nil {
			s.indexMetrics.Observe(float64(i))
		}
		if !f(i, v) {
			break
		}
	}
}

func (s *Slice[T]) Purge() {
	if s.lengthMetrics != nil {
		s.lengthMetrics.Set(0)
	}

	s.locker.Lock()
	defer s.locker.Unlock()
	s.s = nil
}

func (s *Slice[T]) WriteToGob(w io.Writer) error {
	gobSlice := func() []T {
		s.locker.RLock()
		defer s.locker.RUnlock()

		gobSlice := make([]T, 0, len(s.s))
		for _, v := range s.s {
			gobSlice = append(gobSlice, v)
		}

		return gobSlice
	}()

	err := gob.NewEncoder(w).Encode(gobSlice)
	if err != nil {
		return fmt.Errorf("failed to write to gob: %w", err)
	}

	return nil
}

func (s *Slice[T]) LoadFromGob(r io.Reader) error {
	gobSlice := []T{}
	err := gob.NewDecoder(r).Decode(&gobSlice)
	if err != nil {
		return fmt.Errorf("failed to load from gob: %w", err)
	}

	s.locker.Lock()
	defer s.locker.Unlock()

	s.s = gobSlice

	return nil
}

func AllPurge() {
	for _, cache := range cacheMap {
		cache.Purge()
	}
}
