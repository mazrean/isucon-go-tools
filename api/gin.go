package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	isutools "github.com/mazrean/isucon-go-tools"
)

type Gin struct {
	*gin.Engine
}

func WrapGin(g *gin.Engine) *Gin {
	if isutools.Enable {
		g.Use(GinMetricsMiddleware())
	}

	return &Gin{
		Engine: g,
	}
}

func (g *Gin) Run(addrs ...string) error {
	listener, ok, err := unixDomainSock()
	if err != nil {
		return err
	}

	if !ok {
		var addr string
		switch len(addrs) {
		case 0:
			port := os.Getenv("PORT")
			if port != "" {
				log.Printf("Environment variable PORT=\"%s\"\n", port)
				addr = ":" + port
				break
			}
			log.Println("Environment variable PORT is undefined. Using port :8080 by default")
			addr = ":8080"
		case 1:
			addr = addrs[0]
		default:
			panic("too many parameters")
		}

		listener, err = tcpListener(addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
	}

	return g.RunListener(listener)
}

func (g *Gin) RunTLS(addr, certFile, keyFile string) error {
	listener, ok, err := unixDomainSock()
	if err != nil {
		return err
	}

	if !ok {
		listener, err = tcpListener(addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
	}

	// logに若干変化が出るが、ISUCON用ツールなので許容する
	return http.ServeTLS(listener, g.Handler(), certFile, keyFile)
}

func GinMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isutools.Enable {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		method := c.Request.Method

		reqSz := reqSize(c.Request)

		start := time.Now()
		c.Next()
		reqDur := float64(time.Since(start)) / float64(time.Second)

		statusCode := c.Writer.Status()
		resSize := float64(c.Writer.Size())
		strStatusCode := strconv.Itoa(statusCode)

		reqSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(strStatusCode, method, path).Inc()
		resSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(resSize)
	}
}
