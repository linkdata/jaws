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

// assetsFS holds Bootstrap v5.3.8 from https://getbootstrap.com/ (see the package
// doc); keep this version note in sync with doc.go when updating the files.
//
//go:embed assets
var assetsFS embed.FS

// Setup registers embedded Bootstrap static assets under prefix.
//
// It is intended to be passed to [jaws.Jaws.Setup]. Returned URLs should be
// included in the page head through [jaws.Jaws.GenerateHeadHTML]. The prefix may
// be absolute ("/static"), relative ("static") or empty; the registered handler
// paths and the returned URLs are kept identical in all cases.
//
// Setup also registers [http.NotFoundHandler] (404) routes under prefix for the
// bundled bootstrap *.map sourcemap paths, quietly answering devtools probes for
// "bootstrap.bundle.min.js.map" and "bootstrap.min.css.map".
func Setup(jw *jaws.Jaws, handleFn jaws.HandleFunc, prefix string) (urls []*url.URL, err error) {
	var files []*staticserve.StaticServe
	if err = staticserve.WalkDir(assetsFS, "assets/static", func(filename string, ss *staticserve.StaticServe) (err error) {
		files = append(files, ss)
		return
	}); err == nil {
		for _, ss := range files {
			// Build an absolute path so jaws.Setup's makeAbsPath leaves the
			// returned URL unchanged; otherwise a relative prefix would be applied
			// twice and the head URL would diverge from the handler path.
			abspath := staticserve.EnsurePrefixSlash(path.Join(prefix, ss.Name))
			u, e := url.Parse(abspath)
			if e == nil {
				urls = append(urls, u)
				handleFn(staticserve.NormalizeGET(abspath), ss)
			}
			err = errors.Join(err, e)
		}
		// Quietly 404 the predictable devtools source-map probes for the bundled
		// assets; they are served only at their exact content-hashed paths.
		handleFn(staticserve.NormalizeGET(path.Join(prefix, "bootstrap.bundle.min.js.map")), http.NotFoundHandler())
		handleFn(staticserve.NormalizeGET(path.Join(prefix, "bootstrap.min.css.map")), http.NotFoundHandler())
	}
	return
}
