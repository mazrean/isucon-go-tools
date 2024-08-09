package profiler

import (
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/felixge/fgprof"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	if !config.Enable {
		return
	}

	strBlockRate, ok := os.LookupEnv("BLOCK_RATE")
	if ok {
		blockRate, err := strconv.Atoi(strBlockRate)
		if err != nil {
			slog.Error("failed to parse BLOCK_RATE",
				slog.String("BLOCK_RATE", strBlockRate),
				slog.String("error", err.Error()),
			)
		} else {
			runtime.SetBlockProfileRate(blockRate)
		}
	}

	strMutexRate, ok := os.LookupEnv("MUTEX_RATE")
	if ok {
		mutexRate, err := strconv.Atoi(strMutexRate)
		if err != nil {
			slog.Error("failed to parse MUTEX_RATE",
				slog.String("MUTEX_RATE", strMutexRate),
				slog.String("error", err.Error()),
			)
		} else {
			runtime.SetMutexProfileFraction(mutexRate)
		}
	}

	err := setupPyroscope()
	if err != nil {
		slog.Error("failed to setup pyroscope",
			slog.String("error", err.Error()),
		)
	}
}

func Register(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug/fgprof", fgprof.Handler())
}
