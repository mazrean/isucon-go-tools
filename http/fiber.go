package isuhttp

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	isutools "github.com/mazrean/isucon-go-tools"
)

func FiberNew(conf ...fiber.Config) *fiber.App {
	if enableGoJson {
		if len(conf) == 0 {
			conf = []fiber.Config{
				{
					JSONEncoder: json.Marshal,
					JSONDecoder: json.Unmarshal,
				},
			}
		} else {
			conf[0].JSONEncoder = json.Marshal
			conf[0].JSONDecoder = json.Unmarshal
		}
	}

	app := fiber.New(conf...)
	app.Use(FiberMetricsMiddleware)

	listener, ok, err := newUnixDomainSockListener()
	if err != nil {
		log.Printf("failed to init unix domain socket: %s\n", err)
	}

	if ok {
		err := app.Listener(listener)
		if err != nil {
			log.Printf("failed to set listener: %s\n", err)
		}
	}

	return app
}

func FiberMetricsMiddleware(next fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !isutools.Enable {
			return next(c)
		}

		path := c.Path()
		host := c.Hostname()
		method := c.Method()

		reqSz := fastHTTPReqSize(c.Request())

		start := time.Now()
		err := next(c)
		reqDur := float64(time.Since(start)) / float64(time.Second)

		// error handlerがDefaultHTTPErrorHandlerでない場合、正しくない可能性あり
		var (
			statusCode int
			resSize    float64
		)
		if err == nil {
			statusCode = c.Response().StatusCode()
			resSize = float64(c.Response().Header.ContentLength())
		} else {
			var httpError *fiber.Error
			if errors.As(err, &httpError) {
				statusCode = httpError.Code
			} else {
				statusCode = fiber.StatusInternalServerError
			}

			if method == http.MethodHead {
				resSize = 0
			} else {
				resSize = float64(len(err.Error()))
			}
		}
		strStatusCode := strconv.Itoa(statusCode)

		reqSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(strStatusCode, method, host, path).Inc()
		resSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(resSize)

		return nil
	}
}
