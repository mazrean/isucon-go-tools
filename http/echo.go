package isuhttp

import (
	stdjson "encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/sonic/decoder"
	"github.com/bytedance/sonic/encoder"
	"github.com/goccy/go-json"
	"github.com/labstack/echo/v4"
	isutools "github.com/mazrean/isucon-go-tools"
)

var (
	jsonSerializer = Sonic
)

type JSONSerializerType uint8

const (
	Sonic JSONSerializerType = iota
	GoJSON
	StdJson
)

func SetJSONSerializer(newJsonSerializer JSONSerializerType) {
	jsonSerializer = newJsonSerializer
}

func init() {
	strEnableGoJson, ok := os.LookupEnv("GO_JSON")
	if !ok {
		return
	}

	enableGoJson, err := strconv.ParseBool(strEnableGoJson)
	if err != nil {
		log.Printf("failed to parse GO_JSON: %s\n", err)
		return
	}

	if enableGoJson {
		jsonSerializer = GoJSON
	}
}

func EchoSetting(e *echo.Echo) *echo.Echo {
	e.Use(EchoMetricsMiddleware)

	switch jsonSerializer {
	case Sonic:
		e.JSONSerializer = SonicJSONSerializer{}
	case GoJSON:
		e.JSONSerializer = JSONSerializer{}
	}

	listener, ok, err := newUnixDomainSockListener()
	if err != nil {
		log.Printf("failed to init unix domain socket: %s\n", err)
	}

	if ok {
		e.Listener = listener
	}

	return e
}

type JSONSerializer struct{}

func (JSONSerializer) Serialize(c echo.Context, i any, indent string) error {
	enc := json.NewEncoder(c.Response())
	return enc.Encode(i)
}

func (JSONSerializer) Deserialize(c echo.Context, i any) error {
	err := json.NewDecoder(c.Request().Body).Decode(i)

	switch err := err.(type) {
	case *json.UnmarshalTypeError:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unmarshal type error: expected=%v, got=%v, field=%v, offset=%v", err.Type, err.Value, err.Field, err.Offset)).SetInternal(err)
	case *json.SyntaxError:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Syntax error: offset=%v, error=%v", err.Offset, err.Error())).SetInternal(err)
	}

	return err
}

type SonicJSONSerializer struct{}

func (SonicJSONSerializer) Serialize(c echo.Context, i any, indent string) error {
	enc := encoder.NewStreamEncoder(c.Response())
	return enc.Encode(i)
}

func (SonicJSONSerializer) Deserialize(c echo.Context, i any) error {
	err := decoder.NewStreamDecoder(c.Request().Body).Decode(i)

	switch err := err.(type) {
	case *stdjson.InvalidUnmarshalError:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Unmarshal type error: expected=%v", err.Type)).SetInternal(err)
	case decoder.SyntaxError:
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Syntax error: offset=%v, error=%v", err.Pos, err.Error())).SetInternal(err)
	}

	return err
}

func EchoMetricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !isutools.Enable {
			return next(c)
		}

		path := c.Path()
		host := c.Request().Host
		method := c.Request().Method

		reqSz := reqSize(c.Request())

		start := time.Now()
		err := next(c)
		reqDur := float64(time.Since(start)) / float64(time.Second)

		// error handlerがDefaultHTTPErrorHandlerでない場合、正しくない可能性あり
		var (
			statusCode   int
			validResSize = true
			resSize      float64
		)
		if c.Response().Committed || err == nil {
			statusCode = c.Response().Status
			resSize = float64(c.Response().Size)
		} else {
			var httpError *echo.HTTPError
			if errors.As(err, &httpError) {
				statusCode = httpError.Code
			} else {
				statusCode = http.StatusInternalServerError
			}

			if method == http.MethodHead {
				resSize = 0
			} else {
				/*
					実際には*echo.HTTPErrorがjson encodeされたものになるが、
					速度低下が許容量を超えるので妥協する
					ErrorHandlerをいい感じにいじれば解決しそう
				*/
				validResSize = false
			}
		}
		strStatusCode := strconv.Itoa(statusCode)

		reqSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqSz)
		reqDurHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(reqDur)
		reqCounterVec.WithLabelValues(strStatusCode, method, host, path).Inc()

		if validResSize {
			resSizeHistogramVec.WithLabelValues(strStatusCode, method, path).Observe(resSize)
		}

		return nil
	}
}
