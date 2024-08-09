package isulocker

import (
	"sync"
	"sync/atomic"

	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "locker"
)

var lockHistVec = promauto.NewHistogramVec(prometheus.HistogramOpts{
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
	if config.Enable {
		timer := prometheus.NewTimer(lockHistVec.WithLabelValues(v.name, "read"))
		defer timer.ObserveDuration()
	}

	v.locker.RLock()
	defer v.locker.RUnlock()

	f(&v.value)
}

func (v *Value[T]) Write(f func(v *T)) {
	if config.Enable {
		timer := prometheus.NewTimer(lockHistVec.WithLabelValues(v.name, "write"))
		defer timer.ObserveDuration()
	}
	v.locker.Lock()
	defer v.locker.Unlock()

	f(&v.value)
}

type WaitSuccess struct {
	// isRunning
	// 実行中か
	isRunning *atomic.Bool
	// succeeded
	//一度でも成功したかどうか
	// flaseからtrueへしか変わらない
	succeeded *atomic.Bool
	cond      *sync.Cond
}

func NewAfterSuccess() *WaitSuccess {
	isRunning := &atomic.Bool{}
	isRunning.Store(false)

	succeeded := &atomic.Bool{}
	succeeded.Store(false)

	return &WaitSuccess{
		isRunning: isRunning,
		succeeded: succeeded,
		cond:      sync.NewCond(&sync.Mutex{}),
	}
}

func (af *WaitSuccess) Run(before func() bool, after func()) {
	func() {
		af.cond.L.Lock()
		defer af.cond.L.Unlock()

		if af.isRunning.Load() && !af.succeeded.Load() {
			af.cond.Wait()
		}
		af.isRunning.Store(true)
	}()

	if af.succeeded.Load() {
		after()
	} else {
		result := before()

		af.cond.L.Lock()
		defer af.cond.L.Unlock()

		af.succeeded.Store(result)
		af.isRunning.Store(false)
	}

	if af.succeeded.Load() {
		af.cond.Broadcast()
	} else {
		af.cond.Signal()
	}
}
