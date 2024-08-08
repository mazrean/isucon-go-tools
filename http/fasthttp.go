package isuhttp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mazrean/isucon-go-tools/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/internal/config"
	"github.com/valyala/fasthttp"
)

func FastListenAndServe(addr string, handler fasthttp.RequestHandler) error {
	if config.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.Serve(listener, handler)
}

func FastListenAndServeTLS(addr, certFile, keyFile string, handler fasthttp.RequestHandler) error {
	if config.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.ServeTLS(listener, certFile, keyFile, handler)
}

func FastListenAndServeTLSEmbed(addr string, certData, keyData []byte, handler fasthttp.RequestHandler) error {
	if config.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.ServeTLSEmbed(listener, certData, keyData, handler)
}

func FastServerListenAndServe(server *fasthttp.Server, addr string) error {
	if config.Enable {
		server.Handler = FastMetricsMiddleware(server.Handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

func FastServerListenAndServeTLS(server *fasthttp.Server, addr string, certFile, keyFile string) error {
	if config.Enable {
		server.Handler = FastMetricsMiddleware(server.Handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return server.ServeTLS(listener, certFile, keyFile)
}

func FastServerListenAndServeTLSEmbed(server *fasthttp.Server, addr string, certData, keyData []byte) error {
	if config.Enable {
		server.Handler = FastMetricsMiddleware(server.Handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return server.ServeTLSEmbed(listener, certData, keyData)
}

func FastMetricsMiddleware(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		if !config.Enable {
			next(ctx)
			return
		}

		benchmark.Continue()

		path := FilterFunc(string(ctx.Path()))
		host := string(ctx.Host())
		method := string(ctx.Method())

		reqSz := fastHTTPReqSize(&ctx.Request)

		// シナリオ解析用メトリクス
		flowCookieValue := ctx.Request.Header.Cookie("isutools_flow")
		if flowCookieValue != nil {
			flowMethod, flowPath, ok := strings.Cut(string(flowCookieValue), ",")
			if ok {
				flowCounterVec.WithLabelValues(flowMethod, flowPath, method, path).Inc()
			}
		}
		flowCookie := new(fasthttp.Cookie)
		flowCookie.SetKey("isutools_flow")
		flowCookie.SetValue(fmt.Sprintf("%s,%s", method, path))
		flowCookie.SetExpire(time.Now().Add(1 * time.Hour))
		ctx.Response.Header.SetCookie(flowCookie)

		start := time.Now()
		next(ctx)
		reqDur := float64(time.Since(start)) / float64(time.Second)

		statusCode := strconv.Itoa(ctx.Response.StatusCode())

		reqSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(statusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(statusCode, method, host, path).Inc()
		resSizeHistogramVec.WithLabelValues(statusCode, method, path).Observe(float64(ctx.Response.Header.ContentLength()))
	})
}
