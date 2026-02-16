package core

import (
	"net/http"
	"strings"
)

var headerCacheNoCache = []string{"no-cache"}

// ServeHTTP can handle the required JaWS endpoints, which all start with "/jaws/".
func (jw *Jaws) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if len(r.URL.Path) > 6 && strings.HasPrefix(r.URL.Path, "/jaws/") {
		if r.URL.Path[6] == '.' {
			switch r.URL.Path {
			case jw.serveCSS.Name:
				jw.serveCSS.ServeHTTP(w, r)
				return
			case jw.serveJS.Name:
				jw.serveJS.ServeHTTP(w, r)
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
		} else if rq := jw.UseRequest(JawsKeyValue(r.URL.Path[6:]), r); rq != nil {
			rq.ServeHTTP(w, r)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
}
