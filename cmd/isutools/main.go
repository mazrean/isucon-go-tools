package main

import (
	"os"

	"github.com/mazrean/isucon-go-tools/analysers/cache"
	"github.com/mazrean/isucon-go-tools/analysers/db"
	"github.com/mazrean/isucon-go-tools/analysers/echo"
	"github.com/mazrean/isucon-go-tools/analysers/embed"
	"github.com/mazrean/isucon-go-tools/analysers/fasthttp"
	"github.com/mazrean/isucon-go-tools/analysers/fiber"
	"github.com/mazrean/isucon-go-tools/analysers/gin"
	"github.com/mazrean/isucon-go-tools/analysers/http"
	"github.com/mazrean/isucon-go-tools/analysers/importer"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "dbdoc":
			err := dbDoc(os.Args[2:])
			if err != nil {
				panic(err)
			}
		}
	}
	multichecker.Main(
		embed.Analyzer,
		echo.Analyzer,
		gin.Analyzer,
		http.Analyzer,
		fiber.Analyzer,
		fasthttp.Analyzer,
		db.Analyzer,
		cache.Analyzer,
		importer.Analyzer,
	)
}
