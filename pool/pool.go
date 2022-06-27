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
