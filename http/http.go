package isuhttp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	isuhttpgen "github.com/mazrean/isucon-go-tools/v2/http/internal/generate"
	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
)

func ListenAndServe(addr string, handler http.Handler) error {
	if config.Enable {
		handler = StdMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return http.Serve(listener, handler)
}

func ListenAndServeTLS(addr, certFile, keyFile string, handler http.Handler) error {
	if config.Enable {
		handler = StdMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return http.ServeTLS(listener, handler, certFile, keyFile)
}

func ServerListenAndServe(server *http.Server) error {
	if config.Enable {
		server.Handler = StdMetricsMiddleware(server.Handler)
	}

	listener, err := listen(server.Addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

func ServerListenAndServeTLS(server *http.Server, certFile, keyFile string) error {
	if config.Enable {
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
	responseWriterMetrics
}

type responseWriterMetrics struct {
	statusCode int
	resSize    float64
}

func newResponseWriterWithMetrics(w http.ResponseWriter) *responseWriterWithMetrics {
	return &responseWriterWithMetrics{
		ResponseWriter: w,
		responseWriterMetrics: responseWriterMetrics{
			statusCode: 200,
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
	// ServeMuxの場合はラップ済みなのでそのまま返す
	if _, ok := next.(*http.ServeMux); ok {
		return next
	}

	benchmark.Continue()

	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if !config.Enable {
			next.ServeHTTP(res, req)
			return
		}

		var metrics *responseWriterMetrics
		wrappedRes := isuhttpgen.ResponseWriterWrapper(res, func(w http.ResponseWriter) isuhttpgen.ResponseWriter {
			rw := newResponseWriterWithMetrics(w)
			metrics = &rw.responseWriterMetrics
			return rw
		})

		path := getPath(req)
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

		statusCode := strconv.Itoa(metrics.statusCode)

		reqSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(statusCode, method, host, path).Inc()
		resSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(metrics.resSize)
	})
}

type pathCtxKey struct{}

func SetPath(req *http.Request, path string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), pathCtxKey{}, path))
}

func getPath(req *http.Request) string {
	iPath := req.Context().Value(pathCtxKey{})
	if iPath == nil {
		return FilterFunc(req.URL.Path)
	}

	path, ok := iPath.(string)
	if !ok {
		return FilterFunc(req.URL.Path)
	}

	return path
}

func ServerMuxHandle(mux *http.ServeMux, pattern string, handler http.Handler) {
	if !config.Enable {
		mux.Handle(pattern, handler)
		return
	}

	pathPattern := pathPattern(pattern)
	mux.Handle(pattern, http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		req = SetPath(req, pathPattern)
		StdMetricsMiddleware(handler).ServeHTTP(res, req)
	}))
}

func ServerMuxHandleFunc(mux *http.ServeMux, pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if !config.Enable {
		mux.HandleFunc(pattern, handler)
		return
	}

	pathPattern := pathPattern(pattern)
	mux.HandleFunc(pattern, func(res http.ResponseWriter, req *http.Request) {
		req = SetPath(req, pathPattern)

		StdMetricsMiddleware(http.HandlerFunc(handler)).ServeHTTP(res, req)
	})
}

func pathPattern(pattern string) string {
	if _, after, found := strings.Cut(pattern, " "); found {
		return after
	}

	return pattern
}
