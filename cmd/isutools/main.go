package main

import (
	"github.com/mazrean/isucon-go-tools/analysers/embed"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(
		embed.Analyzer,
	)
}
