package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

var (
	Enable = true
	Host   string
	Addr   = ":6060"
)

func init() {
	strEnable, ok := os.LookupEnv("ISUTOOLS_ENABLE")
	if ok {
		enable, err := strconv.ParseBool(strings.TrimSpace(strEnable))
		if err != nil {
			slog.Error("failed to parse ISUTOOLS_ENABLE",
				slog.String("ISUTOOLS_ENABLE", strEnable),
				slog.String("error", err.Error()),
			)
			return
		}

		Enable = enable
	}

	host, ok := os.LookupEnv("ISUTOOLS_HOST_NAME")
	if ok {
		Host = host
	}

	addr, ok := os.LookupEnv("ISUTOOLS_ADDR")
	if ok {
		Addr = addr
	}
}
