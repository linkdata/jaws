package jaws

import (
	"compress/gzip"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

var headerContentGZip = []string{"gzip"}

func TestServeHTTP_GetJavascript(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	is := newTestHelper(t)

	mux := http.NewServeMux()
	mux.Handle("/jaws/", jw)

	req := httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "blepp")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptText))
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".js")})
	is.Equal(w.Header()["Content-Encoding"], nil)

	req = httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".js")})
	is.Equal(w.Header()["Content-Encoding"], headerContentGZip)

	gr, err := gzip.NewReader(w.Body)
	is.NoErr(err)
	b := make([]byte, len(JavascriptText)+32)
	n, err := gr.Read(b)
	b = b[:n]
	is.NoErr(err)
	is.NoErr(gr.Close())
	is.Equal(len(JavascriptText), len(b))
	is.Equal(string(JavascriptText), string(b))
}

func TestServeHTTP_GetCSS(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	is := newTestHelper(t)

	mux := http.NewServeMux()
	mux.Handle("/jaws/", jw)

	req := httptest.NewRequest("", JawsCSSPath, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JawsCSS))
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".css")})
}

func TestServeHTTP_GetPing(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/.ping", nil)
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)
	is.Equal(len(w.Body.Bytes()), 0)
	is.Equal(w.Header()["Content-Length"], nil)
	is.Equal(w.Code, http.StatusNoContent)

	req = httptest.NewRequest(http.MethodPost, "/jaws/.ping", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusMethodNotAllowed)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/jaws/.pong", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/something_else", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	jw.Close()

	req = httptest.NewRequest("", "/jaws/.ping", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusServiceUnavailable)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)
}

func TestServeHTTP_GetKey(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/", nil)
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/jaws/12345678", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	w = httptest.NewRecorder()
	rq := jw.NewRequest(req)
	req = httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
	is.Equal(w.Header()["Cache-Control"], nil)
}

func TestServeHTTP_Noscript(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	w := httptest.NewRecorder()
	rq := jw.NewRequest(httptest.NewRequest("", "/", nil))
	req := httptest.NewRequest("", "/jaws/"+rq.JawsKeyString()+"/noscript", nil)
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNoContent)
}
