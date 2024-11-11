package staticserve

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// HandleFunc matches the signature of http.ServeMux.Handle(), but is called without
// method or parameters for the pattern. E.g. ("/static/filename.1234567.js").
type HandleFunc = func(uri string, handler http.Handler)

func ensurePrefixSlash(s string) string {
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return s
}

func Handle(fpath string, data []byte, handleFn HandleFunc) (uri string, err error) {
	var ss *StaticServe
	if ss, err = New(fpath, data); err == nil {
		uri = ensurePrefixSlash(ss.Name)
		handleFn(uri, ss)
	}
	return
}

func HandleFS(fsys fs.FS, root, fpath string, handleFn HandleFunc) (uri string, err error) {
	var f fs.File
	if f, err = fsys.Open(path.Join(root, fpath)); err == nil {
		defer f.Close()
		var b []byte
		if b, err = io.ReadAll(f); err == nil {
			uri, err = Handle(fpath, b, handleFn)
		}
	}
	return
}
