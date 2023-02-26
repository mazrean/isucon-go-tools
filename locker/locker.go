package locker

import (
	"sync"

	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "locker"
)

var lockHistVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: prometheusNamespace,
	Subsystem: prometheusSubsystem,
	Name:      "index_access",
	Buckets:   prometheus.DefBuckets,
}, []string{"name", "type"})

type Value[T any] struct {
	name   string
	locker sync.RWMutex
	value  T
}

func NewValue[T any](value T, name string) *Value[T] {
	return &Value[T]{
		name:  name,
		value: value,
	}
}

func (v *Value[T]) Read(f func(v *T)) {
	if isutools.Enable {
		timer := prometheus.NewTimer(lockHistVec.WithLabelValues(v.name, "read"))
		v.locker.RLock()
		timer.ObserveDuration()
	} else {
		v.locker.RLock()
	}

	f(&v.value)
	v.locker.RUnlock()
}

func (v *Value[T]) Write(f func(v *T)) {
	if isutools.Enable {
		timer := prometheus.NewTimer(lockHistVec.WithLabelValues(v.name, "write"))
		v.locker.Lock()
		timer.ObserveDuration()
	} else {
		v.locker.Lock()
	}

	f(&v.value)
	v.locker.Unlock()
}

type WaitSuccess struct {
	// isRunning
	// 実行中か
	isRunning bool
	// succeeded
	//一度でも成功したかどうか
	// flaseからtrueへしか変わらない
	succeeded bool
	cond      *sync.Cond
}

func NewAfterSuccess() *WaitSuccess {
	return &WaitSuccess{
		isRunning: false,
		succeeded: false,
		cond:      sync.NewCond(&sync.Mutex{}),
	}
}

func (af *WaitSuccess) Run(before func() bool, after func()) {
	af.cond.L.Lock()
	if af.isRunning && !af.succeeded {
		af.cond.Wait()
	}
	af.isRunning = true
	af.cond.L.Unlock()

	if af.succeeded {
		after()
	} else {
		af.succeeded = before()
	}

	af.cond.L.Lock()
	af.isRunning = false
	af.cond.L.Unlock()

	if af.succeeded {
		af.cond.Broadcast()
	} else {
		af.cond.Signal()
	}
}
