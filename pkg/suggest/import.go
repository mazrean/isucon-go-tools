package suggest

import (
	"go/ast"
)

type ImportInfo struct {
	File  *ast.File
	Ident string
	Path  string
}
