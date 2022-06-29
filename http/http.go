package isuhttp

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	isutools "github.com/mazrean/isucon-go-tools"
)

func ListenAndServe(addr string, handler http.Handler) error {
	if isutools.Enable {
		handler = StdMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return http.Serve(listener, handler)
}

func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	if isutools.Enable {
		handler = StdMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return http.ServeTLS(listener, handler, certFile, keyFile)
}

func ServerListenAndServe(server *http.Server) error {
	if isutools.Enable {
		server.Handler = StdMetricsMiddleware(server.Handler)
	}

	listener, err := listen(server.Addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

func ServerListenAndServeTLS(server *http.Server, certFile, keyFile string) error {
	if isutools.Enable {
		server.Handler = StdMetricsMiddleware(server.Handler)
	}

	listener, err := listen(server.Addr)
	if err != nil {
		return err
	}

	return server.ServeTLS(listener, certFile, keyFile)
}

func listen(addr string) (net.Listener, error) {
	listener, ok, err := newUnixDomainSockListener()
	if err != nil {
		return nil, err
	}

	if !ok {
		if addr == "" {
			addr = ":http"
		}

		listener, err = newTCPListener(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
	}

	return listener, nil
}

type responseWriterWithMetrics struct {
	http.ResponseWriter
	statusCode int
	resSize    float64
}

func newResponseWriterWithMetrics(w http.ResponseWriter) *responseWriterWithMetrics {
	return &responseWriterWithMetrics{
		ResponseWriter: w,
		statusCode:     -1,
		resSize:        0,
	}
}

func (r *responseWriterWithMetrics) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriterWithMetrics) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.resSize += float64(n)

	return n, err
}

func StdMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !isutools.Enable {
			next.ServeHTTP(res, req)
			return
		}

		wrappedRes := newResponseWriterWithMetrics(res)

		path := req.URL.Path
		method := req.Method

		reqSz := reqSize(req)

		start := time.Now()
		next.ServeHTTP(wrappedRes, req)
		reqDur := float64(time.Since(start)) / float64(time.Second)

		if wrappedRes.statusCode == -1 {
			return
		}

		statusCode := strconv.Itoa(wrappedRes.statusCode)

		reqSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(statusCode, method, path).Inc()
		resSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(wrappedRes.resSize)
	})
}
