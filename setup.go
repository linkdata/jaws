package jaws

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/linkdata/staticserve"
)

// HandleFunc matches the signature of [http.ServeMux.Handle].
type HandleFunc = func(pattern string, handler http.Handler)

// SetupFunc is called by [Jaws.Setup] and allows setting up addons for JaWS.
//
// When [Jaws.Setup] is called with a nil [HandleFunc], setup functions receive
// a no-op handler registration function.
//
// The URLs returned will be used in a call to [Jaws.GenerateHeadHTML].
type SetupFunc = func(jw *Jaws, handleFn HandleFunc, prefix string) (urls []*url.URL, err error)

// makeAbsPath returns a copy of u with prefix prepended to relative paths.
//
// When a non-empty prefix is applied, the joined path is slash-rooted. An empty
// prefix preserves a relative URL.
func makeAbsPath(prefix string, u *url.URL) *url.URL {
	if u != nil {
		copied := *u
		u = &copied
		if prefix != "" && u.Scheme == "" && u.Host == "" && !path.IsAbs(u.Path) {
			u.Path = staticserve.EnsurePrefixSlash(path.Join(prefix, u.Path))
		}
	}
	return u
}

// Setup configures [Jaws] with extra functionality and resources.
//
// The list of extras can be strings, [*url.URL], [*staticserve.StaticServe] or
// []*staticserve.StaticServe URL resources, or a setup function matching
// [SetupFunc] such as jawsboot.Setup.
//
// It calls [Jaws.GenerateHeadHTML] with the final list of URLs, with any
// relative URL paths prefixed with prefix.
//
// [staticserve.StaticServe] extras are local resources. Their generated URLs
// are slash-rooted so they match their registered handlers, including when
// prefix is empty. Other relative URL extras remain relative with an empty
// prefix.
//
// If handleFn is nil, Setup generates head HTML from the configured resources
// without registering any handlers.
func (jw *Jaws) Setup(handleFn HandleFunc, prefix string, extras ...any) (err error) {
	var urls []*url.URL
	setupHandleFn := handleFn
	if setupHandleFn == nil {
		setupHandleFn = func(string, http.Handler) {}
	}

	handleStaticServe := func(ss *staticserve.StaticServe) {
		if ss != nil {
			assetPath := ss.Name
			if !path.IsAbs(assetPath) {
				assetPath = path.Join(prefix, assetPath)
			}
			u := &url.URL{Path: path.Join("/", assetPath)}
			urls = append(urls, u)
			if handleFn != nil {
				setupHandleFn(staticserve.NormalizeGET(u.String()), ss)
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
			u, urlErr := url.Parse(extra)
			err = errors.Join(err, urlErr)
			urls = append(urls, makeAbsPath(prefix, u))
		case *url.URL:
			urls = append(urls, makeAbsPath(prefix, extra))
		case *staticserve.StaticServe:
			handleStaticServe(extra)
		case SetupFunc:
			setupURLs, setupErr := extra(jw, setupHandleFn, prefix)
			err = errors.Join(err, setupErr)
			for _, u := range setupURLs {
				urls = append(urls, makeAbsPath(prefix, u))
			}
		default:
			err = errors.Join(err, fmt.Errorf("jaws.Setup: expected a string, *url.URL, *staticserve.StaticServe, []*staticserve.StaticServe or jaws.SetupFunc, not %T", extra))
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
