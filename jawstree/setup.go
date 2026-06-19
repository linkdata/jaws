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
// The prefix may be absolute ("/static"), relative ("static") or empty; the registered
// handler paths and the returned URLs are kept identical in all cases.
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
			// Build an absolute path so jaws.Setup's makeAbsPath leaves the returned
			// URL unchanged and it matches the handler pattern for any prefix form
			// (absolute, relative or empty). Returning the raw path.Join result would
			// let makeAbsPath re-apply a relative or empty prefix, diverging the head
			// URL from the registered handler path; mirror jawsboot.Setup, which
			// documents this.
			abspath := staticserve.EnsurePrefixSlash(path.Join(prefix, ss.Name))
			u, e := url.Parse(abspath)
			if e == nil {
				urls = append(urls, u)
				handleFn(staticserve.NormalizeGET(abspath), ss)
			}
			err = errors.Join(err, e)
		}
		handleFn(http.MethodGet+" "+initScriptPattern, http.HandlerFunc(serveInitScript))
	}
	return
}
