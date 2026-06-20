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
// be absolute ("/static"), relative ("static") or empty; the returned URL path
// and the path component of the registered handler pattern are kept identical in
// all cases.
//
// Setup also registers [http.NotFoundHandler] (404) routes under prefix for the
// bundled bootstrap *.map sourcemap paths, quietly answering devtools probes for
// "bootstrap.bundle.min.js.map" and "bootstrap.min.css.map".
//
// handleFn must not be nil when calling Setup directly; it is invoked once per
// asset. Reaching it through [jaws.Jaws.Setup] is always safe, since that
// substitutes a no-op handler when its own handleFn is nil (see [jaws.SetupFunc]).
// If a returned URL fails to parse, Setup joins the error into err but still
// registers the remaining assets, so a non-nil err may accompany a partially
// populated urls.
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
