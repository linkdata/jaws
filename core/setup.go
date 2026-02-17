package core

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/linkdata/jaws/staticserve"
)

// HandleFunc matches the signature of http.ServeMux.Handle(), but is called without
// method or parameters for the pattern. E.g. ("/static/filename.1234567.js").
type HandleFunc = func(uri string, handler http.Handler)

// SetupFunc is called by Setup and allows setting up addons for JaWS.
//
// The urls returned will be used in a call to GenerateHeadHTML.
type SetupFunc = func(jw *Jaws, handleFn HandleFunc, prefix string) (urls []*url.URL, err error)

// makeAbsPath prepends the prefix to u's path if it is relative.
// Returns the (possibly modified) u.
func makeAbsPath(prefix string, u *url.URL) *url.URL {
	if prefix != "" && u != nil {
		if !path.IsAbs(u.Path) {
			u.Path = path.Join(prefix, u.Path)
		}
	}
	return u
}

// Setup configures Jaws with extra functionality and resources.
//
// The list of extras can be strings, *url.URL or *staticserve.StaticServe (URL resources)
// or a setup function matching jaws.JawsSetupFunc like jawsboot.JawsSetup.
//
// It calls GenerateHeadHTML with the final list of URLs, with any
// relative URL paths prefixed with prefix.
func (jw *Jaws) Setup(handleFn HandleFunc, prefix string, extras ...any) (err error) {
	var urls []*url.URL

	handleStaticServe := func(ss *staticserve.StaticServe) {
		if ss != nil {
			u, uerr := url.Parse(ss.Name)
			err = errors.Join(err, uerr)
			if u != nil {
				u = makeAbsPath(prefix, u)
				urls = append(urls, u)
				if handleFn != nil {
					handleFn(u.String(), ss)
				}
			}
		}
	}

	for _, extra := range extras {
		switch extra := extra.(type) {
		case []*staticserve.StaticServe:
			for _, ss := range extra {
				handleStaticServe(ss)
			}
		case string:
			u, uerr := url.Parse(extra)
			err = errors.Join(err, uerr)
			urls = append(urls, makeAbsPath(prefix, u))
		case *url.URL:
			urls = append(urls, makeAbsPath(prefix, extra))
		case *staticserve.StaticServe:
			handleStaticServe(extra)
		case SetupFunc:
			setupurls, setuperr := extra(jw, handleFn, prefix)
			err = errors.Join(err, setuperr)
			for _, u := range setupurls {
				urls = append(urls, makeAbsPath(prefix, u))
			}
		default:
			panic(fmt.Sprintf("expected a string, *url.URL, *staticserve.StaticServe or jaws.SetupFunc, not %T", extra))
		}
	}
	var extraFiles []string
	for _, u := range urls {
		if u != nil {
			extraFiles = append(extraFiles, u.String())
		}
	}
	err = errors.Join(err, jw.GenerateHeadHTML(extraFiles...))
	return
}
