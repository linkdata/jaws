package jawsboot

import (
	"embed"
	"net/http"
	"path"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/staticserve"
)

// HandleFunc matches the signature of http.ServeMux.Handle(), but is called without
// method or parameters for the pattern. E.g. ("/static/filename.1234567.js").
type HandleFunc = func(uri string, handler http.Handler)

//go:embed assets
var assetsFS embed.FS

// Files returns the staticserve.StaticServe entries for the embedded Bootstrap JS and CSS.
func Files() (files []*staticserve.StaticServe, err error) {
	err = staticserve.WalkDir(assetsFS, "assets/static", func(filename string, ss *staticserve.StaticServe) (err error) {
		files = append(files, ss)
		return
	})
	return
}

// GenerateHeadHTML calls jw.GenerateHeadHTML with URL's for the staticserve files
// prefixed with the given path prefix and any extra URL's you provide.
func GenerateHeadHTML(jw *jaws.Jaws, prefix string, files []*staticserve.StaticServe, extra ...string) (err error) {
	var extraFiles []string
	for _, ss := range files {
		extraFiles = append(extraFiles, path.Join(prefix, ss.Name))
	}
	extraFiles = append(extraFiles, extra...)
	return jw.GenerateHeadHTML(extraFiles...)
}

// SetupUsing sets up Jaws to serve the Bootstrap files from the prefix path,
// calling handleFn for each URI and staticserve.StaticServe.
// If handleFn is nil, http.DefaultServeMux.Handle is used instead.
// Any extra URL's given are passed to GenerateHeadHTML.
func SetupUsing(jw *jaws.Jaws, prefix string, handleFn HandleFunc, extra ...string) (err error) {
	var files []*staticserve.StaticServe
	if handleFn == nil {
		handleFn = http.DefaultServeMux.Handle
	}
	if files, err = Files(); err == nil {
		if err = GenerateHeadHTML(jw, prefix, files, extra...); err == nil {
			for _, ss := range files {
				handleFn(path.Join(prefix, ss.Name), ss)
			}
			handleFn(path.Join(prefix, "bootstrap.bundle.min.js.map"), http.NotFoundHandler())
			handleFn(path.Join(prefix, "bootstrap.min.css.map"), http.NotFoundHandler())
		}
	}
	return
}

// Setup calls SetupUsing with a prefix of "/static".
func Setup(jw *jaws.Jaws, handleFn HandleFunc, extra ...string) (err error) {
	return SetupUsing(jw, "/static", handleFn, extra...)
}
