package isutools

import (
	"log/slog"
	"net/http"

	_ "net/http/pprof"

	isudb "github.com/mazrean/isucon-go-tools/v2/db"
	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	_ "github.com/mazrean/isucon-go-tools/v2/internal/log"
	"github.com/mazrean/isucon-go-tools/v2/profiler"
)

func init() {
	if !config.Enable {
		return
	}

	mux := http.NewServeMux()
	profiler.Register(mux)
	benchmark.Register(mux)
	isudb.Register(mux)

	go func() {
		server := http.Server{
			Addr:    config.Addr,
			Handler: mux,
		}
		err := server.ListenAndServe()
		if err != nil {
			slog.Error("failed to start http server",
				slog.String("addr", config.Addr),
				slog.String("error", err.Error()),
			)
		}
	}()
}
