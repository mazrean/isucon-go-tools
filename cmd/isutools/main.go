package main

import (
	"github.com/mazrean/isucon-go-tools/analysers/db"
	"github.com/mazrean/isucon-go-tools/analysers/echo"
	"github.com/mazrean/isucon-go-tools/analysers/embed"
	"github.com/mazrean/isucon-go-tools/analysers/gin"
	"github.com/mazrean/isucon-go-tools/analysers/http"
	"github.com/mazrean/isucon-go-tools/analysers/importer"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		embed.Analyzer,
		echo.Analyzer,
		gin.Analyzer,
		http.Analyzer,
		db.Analyzer,
		importer.Analyzer,
	)
}
