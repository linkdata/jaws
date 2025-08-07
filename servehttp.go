package jaws

import (
	"net/http"
	"strconv"
	"strings"
)

var headerCacheStatic = []string{"public, max-age=31536000, s-maxage=31536000, immutable"}
var headerCacheNoCache = []string{"no-cache"}
var headerAcceptEncoding = []string{"Accept-Encoding"}
var headerContentTypeJS = []string{"application/javascript; charset=utf-8"}
var headerContentTypeCSS = []string{"text/css; charset=utf-8"}
var headerContentGZip = []string{"gzip"}

// ServeHTTP can handle the required JaWS endpoints, which all start with "/jaws/".
func (jw *Jaws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if len(r.RequestURI) > 6 && strings.HasPrefix(r.RequestURI, "/jaws/") {
		if r.RequestURI[6] == '.' {
			switch r.RequestURI {
			case JawsCSSPath:
				hdr := w.Header()
				hdr["Cache-Control"] = headerCacheStatic
				hdr["Content-Type"] = headerContentTypeCSS
				hdr["Content-Length"] = []string{strconv.Itoa(len(JawsCSS))}
				_, _ = w.Write(JawsCSS) // #nosec G104
				return
			case JavascriptPath:
				hdr := w.Header()
				hdr["Cache-Control"] = headerCacheStatic
				hdr["Content-Type"] = headerContentTypeJS
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
				hdr["Content-Length"] = []string{strconv.Itoa(len(js))}
				_, _ = w.Write(js) // #nosec G104
				return
			case "/jaws/.ping":
				w.Header()["Cache-Control"] = headerCacheNoCache
				select {
				case <-jw.Done():
					w.WriteHeader(http.StatusServiceUnavailable)
				default:
					w.WriteHeader(http.StatusNoContent)
				}
				return
			}
		} else if rq := jw.UseRequest(JawsKeyValue(r.RequestURI[6:]), r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
