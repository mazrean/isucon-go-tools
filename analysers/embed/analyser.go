package embed

import (
	"fmt"

	"github.com/mazrean/isucon-go-tools/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

const (
	pkgName = "github.com/mazrean/isucon-go-tools"
)

var Analyzer = &analysis.Analyzer{
	Name: "embed",
	Doc:  "import github.com/mazrean/isucon-go-tools package",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	mainFile, ok := suggest.FindMainFile(pass)
	if !ok {
		return nil, nil
	}

	err := suggest.ImportPackage(pass, mainFile, "_", pkgName)
	if err != nil {
		return nil, fmt.Errorf("failed to import %s package: %w", pkgName, err)
	}

	return nil, nil
}
