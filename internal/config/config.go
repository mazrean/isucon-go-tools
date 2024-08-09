package config

import (
	"log"
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
			log.Printf("failed to parse ISUTOOLS_ENABLE: %v", err)
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
