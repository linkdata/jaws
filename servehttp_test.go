package jaws

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeHTTP_GetJavascript(t *testing.T) {
	jw := New()
	go jw.Serve()
	defer jw.Close()

	is := testHelper{t}

	mux := http.NewServeMux()
	mux.Handle("/jaws/", jw)

	req := httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "blepp")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptText))
	is.Equal(w.Header()["Cache-Control"], headerCacheStatic)
	is.Equal(w.Header()["Content-Type"], headerContentType)
	is.Equal(w.Header()["Content-Encoding"], nil)

	req = httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptGZip))
	is.Equal(w.Header()["Cache-Control"], headerCacheStatic)
	is.Equal(w.Header()["Content-Type"], headerContentType)
	is.Equal(w.Header()["Content-Encoding"], headerContentGZip)

	req = httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	w = httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptGZip))
	is.Equal(w.Header()["Cache-Control"], headerCacheStatic)
	is.Equal(w.Header()["Content-Type"], headerContentType)
	is.Equal(w.Header()["Content-Encoding"], headerContentGZip)
}

func TestServeHTTP_GetPing(t *testing.T) {
	is := testHelper{t}
	jw := New()
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
	is := testHelper{t}
	jw := New()
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
	rq := jw.NewRequest(w, req)
	req = httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
	is.Equal(w.Header()["Cache-Control"], nil)
}
