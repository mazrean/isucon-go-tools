package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	Enable   = true
	HostName string
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

	hostName, ok := os.LookupEnv("HOST_NAME")
	if ok {
		HostName = hostName
	}
}
