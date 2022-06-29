package embed

import (
	"reflect"

	"github.com/mazrean/isucon-go-tools/pkg/suggest"
	"golang.org/x/tools/go/analysis"
)

const (
	pkgName = "github.com/mazrean/isucon-go-tools"
)

var (
	importPkgs []*suggest.ImportInfo
	Analyzer   = &analysis.Analyzer{
		Name:       "embed",
		Doc:        "import github.com/mazrean/isucon-go-tools package",
		Run:        run,
		ResultType: reflect.TypeOf([]*suggest.ImportInfo{}),
	}
)

func run(pass *analysis.Pass) (any, error) {
	mainFile, ok := suggest.FindMainFile(pass)
	if !ok {
		return nil, nil
	}

	return append(importPkgs, &suggest.ImportInfo{
		File:  mainFile,
		Ident: "_",
		Path:  pkgName,
	}), nil
}
