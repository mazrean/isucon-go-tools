package isuhttp

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mazrean/isucon-go-tools/v2/internal/benchmark"
	"github.com/mazrean/isucon-go-tools/v2/internal/config"
)

func GinNew(engine *gin.Engine) *gin.Engine {
	engine.Use(GinMetricsMiddleware)

	return engine
}

func GinRun(engine *gin.Engine, addrs ...string) error {
	listener, ok, err := newUnixDomainSockListener()
	if err != nil {
		log.Printf("failed to init unix domain socket: %s\n", err)
	}

	if ok {
		return engine.RunListener(listener)
	}

	return engine.Run(addrs...)
}

func GinRunTLS(engine *gin.Engine, addr, certFile, keyFile string) error {
	listener, ok, err := newUnixDomainSockListener()
	if err != nil {
		log.Printf("failed to init unix domain socket: %s\n", err)
	}

	if ok {
		return engine.RunListener(listener)
	}

	// logに若干変化が出るが、ISUCON用ツールなので許容する
	return engine.RunTLS(addr, certFile, keyFile)
}

func GinMetricsMiddleware(c *gin.Context) {
	if !config.Enable {
		c.Next()
		return
	}

	benchmark.Continue()

	path := c.FullPath()
	host := c.Request.Host
	method := c.Request.Method

	reqSz := reqSize(c.Request)

	// シナリオ解析用メトリクス
	flowCookieValue, err := c.Cookie("isutools_flow")
	if err == nil {
		flowMethod, flowPath, ok := strings.Cut(flowCookieValue, ",")
		if ok {
			flowCounterVec.WithLabelValues(flowMethod, flowPath, method, path).Inc()
		}
	}
	c.SetCookie("isutools_flow", fmt.Sprintf("%s,%s", method, path), int(1*time.Hour), "", "", false, true)

	start := time.Now()
	c.Next()
	reqDur := float64(time.Since(start)) / float64(time.Second)

	statusCode := c.Writer.Status()
	resSize := c.Writer.Size()

	strStatusCode := strconv.Itoa(statusCode)

	reqSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqSz)
	reqDurHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqDur)
	reqCounterVec.WithLabelValues(strStatusCode, method, host, path).Inc()
	resSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(float64(resSize))
}
