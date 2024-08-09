package isutools

import (
	"log"
	"net/http"
	"os"

	_ "net/http/pprof"

	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/mazrean/isucon-go-tools/v2/profiler"
)

func init() {
	if !config.Enable {
		return
	}

	addr, ok := os.LookupEnv("ISUTOOL_ADDR")
	if !ok {
		addr = ":6060"
	}

	mux := http.NewServeMux()
	profiler.Register(mux)
	benchmark.Register(mux)

	go func() {
		server := http.Server{
			Addr:    addr,
			Handler: mux,
		}
		err := server.ListenAndServe()
		if err != nil {
			log.Printf("failed to listen and serve(%s): %v", addr, err)
		}
	}()
}
