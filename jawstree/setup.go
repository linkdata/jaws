package jawstree

import (
	"embed"
	"errors"
	"net/http"
	"net/url"
	"path"

	"github.com/linkdata/jaws"
	"github.com/linkdata/staticserve"
)

//go:embed assets
var assetsFS embed.FS

// treeview from https://github.com/stefaneichert/quercus.js

// Setup registers embedded jawstree static assets under prefix.
//
// It is intended to be passed to [jaws.Jaws.Setup]. Returned URLs should be
// included in the page head through [jaws.Jaws.GenerateHeadHTML].
func Setup(jw *jaws.Jaws, handleFn jaws.HandleFunc, prefix string) (urls []*url.URL, err error) {
	var files []*staticserve.StaticServe
	if err = staticserve.WalkDir(assetsFS, "assets", func(filename string, ss *staticserve.StaticServe) (err error) {
		files = append(files, ss)
		return
	}); err == nil {
		for _, ss := range files {
			u, e := url.Parse(path.Join(prefix, ss.Name))
			if e == nil {
				urls = append(urls, u)
				handleFn(http.MethodGet+" "+u.String(), ss)
			}
			err = errors.Join(err, e)
		}
		handleFn(http.MethodGet+" "+initScriptPattern, http.HandlerFunc(serveInitScript))
		handleFn(http.MethodGet+" "+path.Join(prefix, "treeview.js.map"), http.NotFoundHandler())
	}
	return
}
