package jaws

import (
	"net/http"
	"strings"
)

var headerCacheStatic = []string{"public, max-age=31536000, s-maxage=31536000, immutable"}
var headerCacheNoCache = []string{"no-cache"}
var headerAcceptEncoding = []string{"Accept-Encoding"}
var headerContentType = []string{"application/javascript; charset=utf-8"}
var headerContentGZip = []string{"gzip"}

// ServeHTTP can handle the required JaWS endpoints, which all start with "/jaws/".
func (jw *Jaws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && strings.HasPrefix(r.RequestURI, "/jaws/") {
		hdr := w.Header()
		hdr["Cache-Control"] = headerCacheNoCache
		switch r.RequestURI {
		case JavascriptPath:
			hdr["Cache-Control"] = headerCacheStatic
			hdr["Content-Type"] = headerContentType
			hdr["Vary"] = headerAcceptEncoding
			js := JavascriptText
			for _, v := range r.Header["Accept-Encoding"] {
				if v == "gzip" {
					js = JavascriptGZip
					hdr["Content-Encoding"] = headerContentGZip
					break
				}
			}
			_, _ = w.Write(js) // #nosec G104
			return
		case "/jaws/.ping":
			select {
			case <-jw.Done():
				w.WriteHeader(http.StatusServiceUnavailable)
			default:
				w.WriteHeader(http.StatusOK)
			}
			return
		default:
			if rq := jw.UseRequest(JawsKeyValue(strings.TrimPrefix(r.RequestURI, "/jaws/")), r); rq != nil {
				rq.ServeHTTP(w, r)
				return
			}
			// fall through to http.StatusNotFound
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
