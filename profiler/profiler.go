package profiler

import (
	"log"
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
			log.Printf("failed to parse BLOCK_RATE(%s): %v", strBlockRate, err)
		} else {
			runtime.SetBlockProfileRate(blockRate)
		}
	}

	strMutexRate, ok := os.LookupEnv("MUTEX_RATE")
	if ok {
		mutexRate, err := strconv.Atoi(strMutexRate)
		if err != nil {
			log.Printf("failed to parse MUTEX_RATE(%s): %v", strMutexRate, err)
		} else {
			runtime.SetMutexProfileFraction(mutexRate)
		}
	}

	err := pyroscopeStart()
	if err != nil {
		log.Printf("failed to init pyroscope: %v", err)
	}
}

func Register(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/debug/fgprof", fgprof.Handler())
}
