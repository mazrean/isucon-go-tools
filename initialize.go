package isutools

import (
	isucache "github.com/mazrean/isucon-go-tools/cache"
	"github.com/mazrean/isucon-go-tools/internal/benchmark"
)

func BeforeInitialize() {
	isucache.AllPurge()
}

func AfterInitialize() {
	benchmark.Start()
}
