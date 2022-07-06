package isuqueue

import (
	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	prometheusNamespace = "isutools"
	prometheusSubsystem = "queue"
)

var (
	enableMetrics = true
	queueMap      = make(map[string]interface {
		Reset()
	}, 0)
)

func SetEnableMetrics(enable bool) {
	enableMetrics = enable
}

type Channel[T any] struct {
	len      int
	name     string
	metrics  *prometheus.GaugeVec
	sender   chan T
	ch       chan T
	receiver chan T
}

func NewChannel[T any](name string, len int) *Channel[T] {
	var metrics *prometheus.GaugeVec
	if isutools.Enable && enableMetrics {
		metrics = promauto.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: prometheusNamespace,
			Subsystem: prometheusSubsystem,
			Name:      "counter",
		}, []string{"name", "status"})
	}
	channel := Channel[T]{
		len:     len,
		name:    name,
		metrics: metrics,
	}

	newChannel(&channel)

	queueMap[name] = &channel

	return &channel
}

func newChannel[T any](channel *Channel[T]) {
	if channel.metrics != nil {
		sender := make(chan T)
		ch := make(chan T, channel.len)
		receiver := make(chan T)

		go func() {
			for t := range sender {
				channel.metrics.WithLabelValues(channel.name, "in").Inc()
				ch <- t
			}
		}()

		go func() {
			for t := range ch {
				channel.metrics.WithLabelValues(channel.name, "out").Inc()
				receiver <- t
			}
		}()

		channel.sender = sender
		channel.ch = ch
		channel.receiver = receiver
	} else {
		ch := make(chan T, channel.len)

		channel.sender = nil
		channel.ch = ch
		channel.receiver = nil
	}
}

func (c *Channel[T]) Push() chan<- T {
	if c.sender == nil {
		return c.ch
	}

	return c.sender
}

func (c *Channel[T]) Pop() <-chan T {
	if c.receiver == nil {
		return c.ch
	}

	return c.receiver
}

func (c *Channel[T]) Reset() {
	if c.sender != nil {
		close(c.sender)
	}

	close(c.ch)

	if c.receiver != nil {
		close(c.receiver)
	}

	newChannel(c)
}

func AllReset() {
	for _, channel := range queueMap {
		channel.Reset()
	}
}
