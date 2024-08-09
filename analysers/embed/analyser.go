package embed

import (
	"reflect"

	"github.com/mazrean/isucon-go-tools/v2/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

const (
	pkgName = "github.com/mazrean/isucon-go-tools/v2"
)

var (
	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "embed",
		Doc:        "import github.com/mazrean/isucon-go-tools/v2 package",
		Run:        run,
		ResultType: reflect.TypeOf(importPkgs),
	}
)

func run(pass *analysis.Pass) (any, error) {
	mainFile, ok := suggest.FindMainFile(pass)
	if !ok {
		return importPkgs, nil
	}

	return append(importPkgs, &suggest.ImportInfo{
		File:  mainFile,
		Ident: "_",
		Path:  pkgName,
	}), nil
}
