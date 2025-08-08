package jawsboot

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
				handleFn(u.String(), ss)
			}
			err = errors.Join(err, e)
		}
		handleFn(path.Join(prefix, "bootstrap.bundle.min.js.map"), http.NotFoundHandler())
		handleFn(path.Join(prefix, "bootstrap.min.css.map"), http.NotFoundHandler())
	}
	return
}

/*
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
*/
