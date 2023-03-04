package isutools

import (
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"

	_ "net/http/pprof"

	"github.com/felixge/fgprof"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// pprofとprometheusの設定
func init() {
	if !Enable {
		return
	}

	addr, ok := os.LookupEnv("METRICS_ADDR")
	if !ok {
		addr = ":6060"
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

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/debug/fgprof", fgprof.Handler())

	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Printf("failed to listen and serve(%s): %v", addr, err)
		}
	}()
}
