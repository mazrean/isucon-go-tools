package isulocker_test

import (
	"sync"
	"testing"
	"time"

	isucache "github.com/mazrean/isucon-go-tools/cache"
	isulocker "github.com/mazrean/isucon-go-tools/locker"
	"github.com/stretchr/testify/assert"
)

func TestAfterFirst(t *testing.T) {
	af := isulocker.NewAfterSuccess()

	s := isucache.NewSlice[bool]("", 5, 2)
	wg := sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			af.Run(func() bool {
				s.Append(true)
				time.Sleep(1 * time.Second)

				return true
			}, func() {
				s.Append(false)
			})
		}()
	}
	wg.Wait()

	if s.Len() != 5 {
		t.Errorf("s.Len() = %d, want %d", s.Len(), 5)
	}
	v, ok := s.Get(0)
	assert.True(t, ok)
	assert.True(t, v)

	for i := 1; i < 5; i++ {
		v, ok := s.Get(i)
		assert.True(t, ok)
		assert.False(t, v)
	}

	af.Run(func() bool {
		s.Append(true)

		return true
	}, func() {
		s.Append(false)
	})
	v, ok = s.Get(5)
	assert.True(t, ok)
	assert.False(t, v)
}
