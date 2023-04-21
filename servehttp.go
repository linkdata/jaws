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
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	switch r.RequestURI {
	case JavascriptPath:
		hdr := w.Header()
		hdr["Cache-Control"] = headerCacheStatic
		hdr["Content-Type"] = headerContentType
		hdr["Vary"] = headerAcceptEncoding
		js := JavascriptText
		for _, s := range r.Header["Accept-Encoding"] {
			for _, v := range strings.Split(s, ",") {
				if strings.TrimSpace(v) == "gzip" {
					js = JavascriptGZip
					hdr["Content-Encoding"] = headerContentGZip
					break
				}
			}
		}
		_, _ = w.Write(js) // #nosec G104
		return
	case "/jaws/.ping":
		w.Header()["Cache-Control"] = headerCacheNoCache
		select {
		case <-jw.Done():
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
		return
	}
	if rq := jw.UseRequest(JawsKeyValue(strings.TrimPrefix(r.RequestURI, "/jaws/")), r); rq != nil {
		rq.ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusNotFound)
}
