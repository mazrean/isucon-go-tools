package isutools

import (
	isucache "github.com/mazrean/isucon-go-tools/v2/cache"
	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
)

func BeforeInitialize() {
	isucache.AllPurge()
}

func AfterInitialize() {
	benchmark.Start()
}
