package isuhttp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	isutools "github.com/mazrean/isucon-go-tools"
	isuhttpgen "github.com/mazrean/isucon-go-tools/http/internal/generate"
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
	reponseWriterMetrics
}

type reponseWriterMetrics struct {
	statusCode int
	resSize    float64
}

func newResponseWriterWithMetrics(w http.ResponseWriter) *responseWriterWithMetrics {
	return &responseWriterWithMetrics{
		ResponseWriter: w,
		reponseWriterMetrics: reponseWriterMetrics{
			statusCode: -1,
			resSize:    0,
		},
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

func (r *responseWriterWithMetrics) CloseNotify() <-chan bool {
	return r.ResponseWriter.(http.CloseNotifier).CloseNotify()
}

func (r *responseWriterWithMetrics) Flush() {
	r.ResponseWriter.(http.Flusher).Flush()
}

func (r *responseWriterWithMetrics) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return r.ResponseWriter.(http.Hijacker).Hijack()
}

func (r *responseWriterWithMetrics) ReadFrom(src io.Reader) (int64, error) {
	return r.ResponseWriter.(io.ReaderFrom).ReadFrom(src)
}

func StdMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !isutools.Enable {
			next.ServeHTTP(res, req)
			return
		}

		var metrics *reponseWriterMetrics
		wrappedRes := isuhttpgen.ResponseWriterWrapper(res, func(w http.ResponseWriter) isuhttpgen.ResponseWriter {
			rw := newResponseWriterWithMetrics(w)
			metrics = &rw.reponseWriterMetrics
			return rw
		})

		path := FilterFunc(req.URL.Path)
		host := req.Host
		method := req.Method

		reqSz := reqSize(req)

		// シナリオ解析用メトリクス
		flowCookie, err := req.Cookie("isutools_flow")
		if err == nil {
			flowMethod, flowPath, ok := strings.Cut(flowCookie.Value, ",")
			if ok {
				flowCounterVec.WithLabelValues(flowMethod, flowPath, method, path).Inc()
			}
		} else {
			flowCookie = new(http.Cookie)
			flowCookie.Name = "isutools_flow"
		}
		flowCookie.Value = fmt.Sprintf("%s,%s", method, path)
		flowCookie.Expires = time.Now().Add(1 * time.Hour)
		http.SetCookie(res, flowCookie)

		start := time.Now()
		next.ServeHTTP(wrappedRes, req)
		reqDur := float64(time.Since(start)) / float64(time.Second)

		if metrics.statusCode == -1 {
			return
		}

		statusCode := strconv.Itoa(metrics.statusCode)

		reqSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(statusCode, method, host, path).Inc()
		resSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(metrics.resSize)
	})
}
