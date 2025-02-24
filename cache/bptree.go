package isucache

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/exp/constraints"
)

var _ = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: prometheusNamespace,
		Subsystem: prometheusSubsystem,
		Name:      "tree_height",
		Help:      "The height of the tree",
	},
	[]string{"name"},
)

type BPTree[K any, V any] struct {
	name       string
	comparator func(K, K) int8
	order      int
	locker     sync.RWMutex
	root       *atomic.Pointer[node[K, V]]
	size       *atomic.Int64
}

type node[K any, V any] struct {
	parent   *node[K, V]
	children []*node[K, V]
	locker   sync.RWMutex
	entries  []*entry[K, V]
	right    *atomic.Pointer[node[K, V]]
	left     *atomic.Pointer[node[K, V]]
}

type entry[K any, V any] struct {
	key    K
	locker sync.RWMutex
	value  V
}

func NewBPTree[V any, K any](name string, order int, comparator func(K, K) int8) (*BPTree[K, V], error) {
	if order < 2 {
		return nil, errors.New("order must be greater than 2")
	}

	root := &atomic.Pointer[node[K, V]]{}
	root.Store(&node[K, V]{
		entries: []*entry[K, V]{},
		right:   &atomic.Pointer[node[K, V]]{},
		left:    &atomic.Pointer[node[K, V]]{},
	})

	t := &BPTree[K, V]{
		name:       name,
		comparator: comparator,
		order:      order,
		locker:     sync.RWMutex{},
		root:       root,
		size:       &atomic.Int64{},
	}

	cacheMap[name] = t

	return t, nil
}

func NewBPTreeWithOrdered[K constraints.Ordered, V any](name string, order int) (*BPTree[K, V], error) {
	return NewBPTree[V](name, order, func(k1, k2 K) int8 {
		switch {
		case k1 < k2:
			return -1
		case k1 > k2:
			return 1
		default:
			return 0
		}
	})
}

func NewBPTreeWithTime[V any](name string, order int) (*BPTree[time.Time, V], error) {
	return NewBPTree[V](name, order, func(k1, k2 time.Time) int8 {
		switch {
		case k1.Before(k2):
			return -1
		case k1.After(k2):
			return 1
		default:
			return 0
		}
	})
}

func (t *BPTree[K, V]) Purge() {
	t.root.Store(&node[K, V]{
		entries: []*entry[K, V]{},
		right:   &atomic.Pointer[node[K, V]]{},
		left:    &atomic.Pointer[node[K, V]]{},
	})
	t.size.Store(0)
}

func (t *BPTree[K, V]) Len() int64 {
	return t.size.Load()
}

func (t *BPTree[K, V]) Store(key K, value V) {
	n, isEnd := t.root.Load(), false
	for !isEnd {
		func() {
			locker := &n.locker
			locker.Lock()
			defer func() {
				locker.Unlock()
			}()

			pos, found := t.search(n, key)
			if found {
				isEnd = true
				n.entries[pos].locker.Lock()
				defer n.entries[pos].locker.Unlock()
				n.entries[pos].value = value
				return
			}

			if len(n.children) > 0 {
				n = n.children[pos]
				return
			}

			t.size.Add(1)
			isEnd = true
			e := &entry[K, V]{
				key:   key,
				value: value,
			}

			nd := n
			for {
				if len(nd.entries) < t.order-1 {
					nd.entries = append(nd.entries, nil)
					copy(nd.entries[pos+1:], nd.entries[pos:])
					nd.entries[pos] = e
					return
				}

				var left, right *node[K, V]
				e, left, right = nd.insertSplit(pos, e)

				if nd.parent == nil {
					root := &node[K, V]{
						entries:  []*entry[K, V]{e},
						children: []*node[K, V]{left, right},
						right:    &atomic.Pointer[node[K, V]]{},
						left:     &atomic.Pointer[node[K, V]]{},
					}
					left.parent, right.parent = root, root
					t.root.Store(root)
					return
				}

				nd = nd.parent
				nd.locker.Lock()
				locker.Unlock()
				locker = &nd.locker
				pos, _ = t.search(nd, e.key)
			}
		}()
	}
}

func (t *BPTree[K, V]) Load(key K) (V, bool) {
	var v V
	ok, isEnd := false, false
	n := t.root.Load()
	for !isEnd {
		func() {
			n.locker.RLock()
			defer n.locker.RUnlock()

			pos, found := t.search(n, key)
			if found {
				ok = true
				n.entries[pos].locker.RLock()
				defer n.entries[pos].locker.RUnlock()
				v = n.entries[pos].value
				return
			}

			if len(n.children) == 0 {
				isEnd = true
				return
			}

			n = n.children[pos]
		}()

		if ok {
			return v, true
		}
	}

	return v, false
}

func (t *BPTree[K, V]) Slice(start, end K) []V {
	if t.comparator(start, end) > 0 {
		return nil
	}

	s := []V{}

	isEnd := false
	n := t.root.Load()
	for !isEnd {
		func() {
			n.locker.RLock()
			defer n.locker.RUnlock()

			pos, _ := t.search(n, start)

			if len(n.children) == 0 {
				isEnd = true
				return
			}

			n = n.children[pos]
		}()
	}

	for n != nil {
		func() {
			n.locker.RLock()
			defer n.locker.RUnlock()

			for _, e := range n.entries {
				if t.comparator(e.key, end) > 0 {
					n = nil
					return
				}

				func() {
					e.locker.RLock()
					defer e.locker.RUnlock()

					s = append(s, e.value)
				}()
			}

			n = n.right.Load()
		}()
	}

	return s
}

func (t *BPTree[K, V]) ReverseSlice(start, end K) []V {
	if t.comparator(start, end) < 0 {
		return nil
	}

	s := []V{}

	isEnd := false
	n := t.root.Load()
	for !isEnd {
		func() {
			n.locker.RLock()
			defer n.locker.RUnlock()

			pos, _ := t.search(n, start)

			if len(n.children) == 0 {
				isEnd = true
				return
			}

			n = n.children[pos]
		}()
	}

	for n != nil {
		func() {
			n.locker.RLock()
			defer n.locker.RUnlock()

			for i := len(n.entries) - 1; i >= 0; i-- {
				if t.comparator(n.entries[i].key, end) < 0 {
					n = nil
					return
				}

				func() {
					n.entries[i].locker.RLock()
					defer n.entries[i].locker.RUnlock()

					s = append(s, n.entries[i].value)
				}()
			}

			n = n.left.Load()
		}()
	}

	return s
}

func (t *BPTree[K, V]) search(n *node[K, V], key K) (int, bool) {
	i := sort.Search(len(n.entries), func(i int) bool {
		return t.comparator(key, n.entries[i].key) >= 0
	})
	found := i < len(n.entries) && t.comparator(key, n.entries[i].key) == 0

	return i, found
}

func (n *node[K, V]) insertSplit(pos int, e *entry[K, V]) (*entry[K, V], *node[K, V], *node[K, V]) {
	left := &node[K, V]{
		parent:  n.parent,
		entries: make([]*entry[K, V], 0, len(n.entries)),
		right:   &atomic.Pointer[node[K, V]]{},
		left:    &atomic.Pointer[node[K, V]]{},
	}
	right := &node[K, V]{
		parent:  n.parent,
		entries: make([]*entry[K, V], 0, len(n.entries)),
		right:   &atomic.Pointer[node[K, V]]{},
		left:    &atomic.Pointer[node[K, V]]{},
	}

	var parentEntry *entry[K, V]
	middle := len(n.entries) / 2
	count := 0
	for i := 0; i < len(n.entries); i++ {
		if i == pos {
			switch {
			case count < middle:
				left.entries = append(left.entries, e)
			case count == middle:
				parentEntry = e
				fallthrough
			case count > middle:
				right.entries = append(right.entries, e)
			}
			count++
		}

		switch {
		case count < middle:
			left.entries = append(left.entries, n.entries[i])
		case count == middle:
			parentEntry = n.entries[i]
			fallthrough
		case count > middle:
			right.entries = append(right.entries, n.entries[i])
		}
		count++
	}

	if pos == len(n.entries) {
		right.entries = append(right.entries, e)
	}

	if len(n.children) == 0 {
		left.left.Store(n.left.Load())
		left.right.Store(right)
		right.left.Store(left)
		right.right.Store(n.right.Load())

		n.left.Load().right.Store(left)
		n.right.Load().left.Store(right)
	}

	return parentEntry, left, right
}
