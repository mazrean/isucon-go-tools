package isutools

import (
	"log"
	"net/http"
	"os"

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

	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/debug/fgprof", fgprof.Handler())

	go func() {
		err := http.ListenAndServe(addr, nil)
		if err != nil {
			log.Printf("failed to listen and serve(%s): %v", addr, err)
		}
	}()
}
