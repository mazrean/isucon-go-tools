package isuhttpgen

import (
	"io"
	"net/http"
)

//go:generate go run github.com/mazrean/iwrapper -src=$GOFILE -dst=iwrapper_gen.go

//iwrapper:target
type ResponseWriter interface {
	//iwrapper:require
	http.ResponseWriter
	http.CloseNotifier
	http.Flusher
	http.Hijacker
	io.ReaderFrom
}
