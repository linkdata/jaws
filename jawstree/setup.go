package jawstree

import (
	"embed"
	"errors"
	"net/http"
	"net/url"
	"path"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/staticserve"
)

//go:embed assets
var assetsFS embed.FS

// treeview from https://github.com/stefaneichert/quercus.js

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
				handleFn(u.String(), ss)
			}
			err = errors.Join(err, e)
		}
		handleFn(path.Join(prefix, "treeview.js.map"), http.NotFoundHandler())
	}
	return
}
