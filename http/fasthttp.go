package isuhttp

import (
	"strconv"
	"time"

	isutools "github.com/mazrean/isucon-go-tools"
	"github.com/valyala/fasthttp"
)

func FastListenAndServe(addr string, handler fasthttp.RequestHandler) error {
	if isutools.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.Serve(listener, handler)
}

func FastListenAndServeTLS(addr, certFile, keyFile string, handler fasthttp.RequestHandler) error {
	if isutools.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.ServeTLS(listener, certFile, keyFile, handler)
}

func FastListenAndServeTLSEmbed(addr string, certData, keyData []byte, handler fasthttp.RequestHandler) error {
	if isutools.Enable {
		handler = FastMetricsMiddleware(handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return fasthttp.ServeTLSEmbed(listener, certData, keyData, handler)
}

func FastServerListenAndServe(server *fasthttp.Server, addr string) error {
	if isutools.Enable {
		server.Handler = FastMetricsMiddleware(server.Handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return server.Serve(listener)
}

func FastServerListenAndServeTLS(server *fasthttp.Server, addr string, certFile, keyFile string) error {
	if isutools.Enable {
		server.Handler = FastMetricsMiddleware(server.Handler)
	}

	listener, err := listen(addr)
	if err != nil {
		return err
	}

	return server.ServeTLS(listener, certFile, keyFile)
}

func FastServerListenAndServeTLSEmbed(server *fasthttp.Server, addr string, certData, keyData []byte) error {
	if isutools.Enable {
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
		if !isutools.Enable {
			next(ctx)
			return
		}

		path := string(ctx.Path())
		host := string(ctx.Host())
		method := string(ctx.Method())

		reqSz := fastHTTPReqSize(&ctx.Request)

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
