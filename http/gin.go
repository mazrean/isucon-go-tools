package isuhttp

import (
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	isutools "github.com/mazrean/isucon-go-tools"
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
	if !isutools.Enable {
		c.Next()
		return
	}

	path := c.FullPath()
	host := c.Request.Host
	method := c.Request.Method

	reqSz := reqSize(c.Request)

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
