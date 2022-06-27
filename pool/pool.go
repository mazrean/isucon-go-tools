package pool

import (
	"sync"

	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "pool"
)

type Pool[T ~*S, S any] struct {
	pool    *sync.Pool
	name    string
	counter *prometheus.CounterVec
}

func New[T ~*S, S any](name string, fn func() T) *Pool[T, S] {
	var counter *prometheus.CounterVec
	if isutools.Enable {
		counter = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "count",
		}, []string{"name", "type"})
	}

	return &Pool[T, S]{
		pool: &sync.Pool{
			New: func() interface{} {
				if counter != nil {
					counter.WithLabelValues(name, "alloc").Inc()
				}

				return fn()
			},
		},
		name:    name,
		counter: counter,
	}
}

func (p *Pool[T, _]) Get() T {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "get").Inc()
	}

	return p.pool.Get().(T)
}

func (p *Pool[T, _]) Put(t T) {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "put").Inc()
	}

	p.pool.Put(t)
}

type SlicePool[T ~*[]S, S any] struct {
	pool    *sync.Pool
	name    string
	counter *prometheus.CounterVec
}

func NewSlice[T ~*[]S, S any](name string, fn func() T) *SlicePool[T, S] {
	var counter *prometheus.CounterVec
	if isutools.Enable {
		counter = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "count",
		}, []string{"name", "type"})
	}

	return &SlicePool[T, S]{
		pool: &sync.Pool{
			New: func() interface{} {
				if counter != nil {
					counter.WithLabelValues(name, "alloc").Inc()
				}

				return fn()
			},
		},
		name:    name,
		counter: counter,
	}
}

func (p *SlicePool[T, _]) Get() T {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "get").Inc()
	}

	// capはそのままで、lenを0にして、データを消去する
	v := p.pool.Get().(T)
	*v = (*v)[:0]

	return v
}

func (p *SlicePool[T, _]) Put(t T) {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "put").Inc()
	}

	p.pool.Put(t)
}

type MapPool[T ~*map[K]S, K comparable, S any] struct {
	pool    *sync.Pool
	name    string
	counter *prometheus.CounterVec
}

func NewMap[T ~*map[K]S, K comparable, S any](name string, fn func() T) *MapPool[T, K, S] {
	var counter *prometheus.CounterVec
	if isutools.Enable {
		counter = promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "count",
		}, []string{"name", "type"})
	}

	return &MapPool[T, K, S]{
		pool: &sync.Pool{
			New: func() interface{} {
				if counter != nil {
					counter.WithLabelValues(name, "alloc").Inc()
				}

				return fn()
			},
		},
		name:    name,
		counter: counter,
	}
}

func (p *MapPool[T, _, _]) Get() T {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "get").Inc()
	}

	// 全てのデータを消去する
	v := p.pool.Get().(T)
	for k := range *v {
		delete(*v, k)
	}

	return v
}

func (p *MapPool[T, _, _]) Put(t T) {
	if p.counter != nil {
		p.counter.WithLabelValues(p.name, "put").Inc()
	}

	p.pool.Put(t)
}
