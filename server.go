package isutools

import (
	"log"
	"net/http"

	_ "net/http/pprof"

	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/mazrean/isucon-go-tools/v2/profiler"
)

func init() {
	if !config.Enable {
		return
	}

	mux := http.NewServeMux()
	profiler.Register(mux)
	benchmark.Register(mux)

	go func() {
		server := http.Server{
			Addr:    config.Addr,
			Handler: mux,
		}
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("failed to listen and serve(%s): %v", config.Addr, err)
		}
	}()
}
