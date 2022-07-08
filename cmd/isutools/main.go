package main

import (
	"github.com/mazrean/isucon-go-tools/analysers/cache"
	"github.com/mazrean/isucon-go-tools/analysers/db"
	"github.com/mazrean/isucon-go-tools/analysers/echo"
	"github.com/mazrean/isucon-go-tools/analysers/embed"
	"github.com/mazrean/isucon-go-tools/analysers/fiber"
	"github.com/mazrean/isucon-go-tools/analysers/gin"
	"github.com/mazrean/isucon-go-tools/analysers/http"
	"github.com/mazrean/isucon-go-tools/analysers/importer"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
)

func main() {
	multichecker.Main(
		fieldalignment.Analyzer,
		embed.Analyzer,
		echo.Analyzer,
		gin.Analyzer,
		http.Analyzer,
		fiber.Analyzer,
		db.Analyzer,
		cache.Analyzer,
		importer.Analyzer,
	)
}
