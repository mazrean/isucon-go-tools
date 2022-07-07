package isucache_test

import (
	"sync/atomic"
	"testing"

	isutools "github.com/mazrean/isucon-go-tools"
	isucache "github.com/mazrean/isucon-go-tools/cache"
)

func init() {
	isutools.Enable = false
}

func BenchmarkMapStoreBalanced(b *testing.B) {
	const hits, misses = 128, 128

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkMapStoreMostlyHits(b *testing.B) {
	const hits, misses = 1023, 1

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkMapStoreMostlyMiss(b *testing.B) {
	const hits, misses = 1, 1023

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkAtomicMapStoreBalanced(b *testing.B) {
	const hits, misses = 128, 128

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkAtomicMapStoreMostlyHits(b *testing.B) {
	const hits, misses = 1023, 1

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkAtomicMapStoreMostlyMiss(b *testing.B) {
	const hits, misses = 1, 1023

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Store(id%(hits+misses), &id)
		}
	})
}

func BenchmarkMapLoadBalanced(b *testing.B) {
	const hits, misses = 128, 128

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}

func BenchmarkMapLoadMostlyHits(b *testing.B) {
	const hits, misses = 1023, 1

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}

func BenchmarkMapLoadMostlyMiss(b *testing.B) {
	const hits, misses = 1, 1023

	m := isucache.NewMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}

func BenchmarkAtomicMapLoadBalanced(b *testing.B) {
	const hits, misses = 128, 128

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}

func BenchmarkAtomicMapLoadMostlyHits(b *testing.B) {
	const hits, misses = 1023, 1

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}

func BenchmarkAtomicMapLoadMostlyMiss(b *testing.B) {
	const hits, misses = 1, 1023

	m := isucache.NewAtomicMap[int, *int]("")
	for i := 0; i < hits; i++ {
		m.Store(i, &i)
	}
	for i := 0; i < hits*2; i++ {
		m.Load(i % hits)
	}

	var i int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(atomic.AddInt64(&i, 1) - 1)
		for ; pb.Next(); id++ {
			m.Load(id % (hits + misses))
		}
	})
}
