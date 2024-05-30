package staticserve

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
)

var headerCacheControl = []string{"public, max-age=31536000, s-maxage=31536000, immutable"}
var headerVary = []string{"Accept-Encoding"}
var headerContentEncoding = []string{"gzip"}

func acceptsGzip(hdr http.Header) bool {
	for _, s := range hdr["Accept-Encoding"] {
		if strings.Contains(s, "gzip") {
			return true
		}
	}
	return false
}

func (ss *StaticServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body io.Reader
	statusCode := http.StatusMethodNotAllowed
	if r.Method == http.MethodGet {
		hdr := w.Header()
		if acceptsGzip(r.Header) {
			body = bytes.NewReader(ss.Gz)
			hdr["Content-Encoding"] = headerContentEncoding
			hdr["Content-Length"] = []string{strconv.Itoa(len(ss.Gz))}
		} else {
			statusCode = http.StatusInternalServerError
			if gzr, err := gzip.NewReader(bytes.NewReader(ss.Gz)); err == nil {
				defer gzr.Close()
				body = gzr
			}
		}
		if body != nil {
			statusCode = http.StatusOK
			hdr["Cache-Control"] = headerCacheControl
			hdr["Vary"] = headerVary
			if ss.ContentType != "" {
				hdr["Content-Type"] = []string{ss.ContentType}
			}
		}
	}
	w.WriteHeader(statusCode)
	if body != nil {
		_, _ = io.Copy(w, body)
	}
}
