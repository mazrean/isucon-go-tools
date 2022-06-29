package isuhttp

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func GinRun(engine *gin.Engine, addrs ...string) error {
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

	// logに若干変化が出るが、ISUCON用ツールなので許容する
	return ListenAndServe(addr, engine.Handler())
}

func GinRunTLS(engine *gin.Engine, addr, certFile, keyFile string) error {
	// logに若干変化が出るが、ISUCON用ツールなので許容する
	return ListenAndServeTLS(addr, certFile, keyFile, engine.Handler())
}
