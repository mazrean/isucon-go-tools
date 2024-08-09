package main

import (
	"os"

	"github.com/mazrean/isucon-go-tools/v2/analysers/cache"
	"github.com/mazrean/isucon-go-tools/v2/analysers/db"
	"github.com/mazrean/isucon-go-tools/v2/analysers/echo"
	"github.com/mazrean/isucon-go-tools/v2/analysers/embed"
	"github.com/mazrean/isucon-go-tools/v2/analysers/fasthttp"
	"github.com/mazrean/isucon-go-tools/v2/analysers/fiber"
	"github.com/mazrean/isucon-go-tools/v2/analysers/gin"
	"github.com/mazrean/isucon-go-tools/v2/analysers/http"
	"github.com/mazrean/isucon-go-tools/v2/analysers/importer"
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

			return
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
