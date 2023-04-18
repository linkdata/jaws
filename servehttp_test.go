package jaws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matryer/is"
)

func TestServeHTTP_GetJavascript(t *testing.T) {
	is := is.New(t)
	jw := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", JavascriptPath, nil)
	w := httptest.NewRecorder()

	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptText))
	is.Equal(w.Header()["Cache-Control"], headerCacheStatic)
	is.Equal(w.Header()["Content-Type"], headerContentType)

	req = httptest.NewRequest("", JavascriptPath, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptGZip))
	is.Equal(w.Header()["Cache-Control"], headerCacheStatic)
	is.Equal(w.Header()["Content-Type"], headerContentType)
	is.Equal(w.Header()["Content-Encoding"], headerContentGZip)
}

func TestServeHTTP_GetPing(t *testing.T) {
	is := is.New(t)
	jw := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/.ping", nil)
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)

	req = httptest.NewRequest(http.MethodPost, "/jaws/.ping", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], nil)

	req = httptest.NewRequest("", "/jaws/.pong", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)

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
	is := is.New(t)
	jw := New()
	go jw.Serve()
	defer jw.Close()

	req := httptest.NewRequest("", "/jaws/not-a-proper-key", nil)
	w := httptest.NewRecorder()

	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)

	req = httptest.NewRequest("", "/jaws/12345678", nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)
	is.Equal(w.Header()["Cache-Control"], headerCacheNoCache)

	rq := jw.NewRequest(context.Background(), req)
	req = httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
}
