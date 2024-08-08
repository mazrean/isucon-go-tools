package isutools

import (
	"log"
	"os"
	"strconv"
	"strings"
)

var (
	Enable = true
)

/*
ファイル名が0.goなため、最初にこのinit()が呼ばれる
ref: https://go.dev/ref/spec#Package_initialization:~:text=To%20ensure%20reproducible%20initialization%20behavior%2C%20build%20systems%20are%20encouraged%20to%20present%20multiple%20files%20belonging%20to%20the%20same%20package%20in%20lexical%20file%20name%20order%20to%20a%20compiler.
*/
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
}
