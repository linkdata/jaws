package staticserve

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"

	"github.com/linkdata/jaws/lib/routepattern"
)

// HandleFunc matches the signature of http.ServeMux.Handle().
//
// Handle and HandleFS pass method-aware patterns. Bare path patterns are normalized to GET.
type HandleFunc = func(uri string, handler http.Handler)

// Handle creates a new StaticServe for the fpath that returns the data given.
// Returns the URI of the resource.
func Handle(fpath string, data []byte, handleFn HandleFunc) (uri string, err error) {
	var ss *StaticServe
	if ss, err = New(fpath, data); err == nil {
		uri = routepattern.EnsurePrefixSlash(ss.Name)
		handleFn(routepattern.NormalizeGET(uri), ss)
	}
	return
}

// HandleFS creates StaticServe handlers for the filepaths given.
// Returns the URI(s) of the resources. If an error occurs, the URI
// of the failed resource will be the empty string.
func HandleFS(fsys fs.FS, handleFn HandleFunc, root string, filepaths ...string) (uris []string, err error) {
	for _, filepath := range filepaths {
		var uri string
		f, ferr := fsys.Open(path.Join(root, filepath))
		if ferr == nil {
			var b []byte
			if b, ferr = io.ReadAll(f); ferr == nil {
				uri, ferr = Handle(filepath, b, handleFn)
			}
			ferr = errors.Join(ferr, f.Close())
		}
		uris = append(uris, uri)
		err = errors.Join(err, ferr)
	}
	return
}
