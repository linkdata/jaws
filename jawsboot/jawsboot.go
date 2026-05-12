package jawsboot

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

// Setup registers embedded Bootstrap static assets under prefix.
//
// It is intended to be passed to [jaws.Jaws.Setup]. Returned URLs should be
// included in the page head through [jaws.Jaws.GenerateHeadHTML].
func Setup(jw *jaws.Jaws, handleFn jaws.HandleFunc, prefix string) (urls []*url.URL, err error) {
	var files []*staticserve.StaticServe
	if err = staticserve.WalkDir(assetsFS, "assets/static", func(filename string, ss *staticserve.StaticServe) (err error) {
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
		handleFn(http.MethodGet+" "+path.Join(prefix, "bootstrap.bundle.min.js.map"), http.NotFoundHandler())
		handleFn(http.MethodGet+" "+path.Join(prefix, "bootstrap.min.css.map"), http.NotFoundHandler())
	}
	return
}
