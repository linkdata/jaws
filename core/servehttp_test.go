package core

import (
	"compress/gzip"
	"errors"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

var headerContentGZip = []string{"gzip"}

type errResponseWriter struct {
	code      int
	header    http.Header
	writeErr  error
	writeCall int
}

func (w *errResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *errResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func (w *errResponseWriter) Write(p []byte) (int, error) {
	w.writeCall++
	return 0, w.writeErr
}

func TestServeHTTP_GetJavascript(t *testing.T) {
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	is := newTestHelper(t)

	mux := http.NewServeMux()
	mux.Handle("GET /jaws/", jw)

	req := httptest.NewRequest("", jw.serveJS.Name, nil)
	req.Header.Add("Accept-Encoding", "blepp")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(JavascriptText))
	is.Equal(w.Header()["Cache-Control"], staticserve.HeaderCacheControl)
	is.Equal(w.Header()["Content-Type"], []string{mime.TypeByExtension(".js")})
	is.Equal(w.Header()["Content-Encoding"], nil)

	req = httptest.NewRequest("", jw.serveJS.Name, nil)
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
	mux.Handle("GET /jaws/", jw)

	req := httptest.NewRequest("", jw.serveCSS.Name, nil)
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
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
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
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
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

func TestServeHTTP_TailScript(t *testing.T) {
	is := newTestHelper(t)
	NextJid = 0
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	item := &testUi{}
	e := rq.NewElement(item)
	e.SetAttr("title", `</script><img onerror=alert(1) src=x>`)
	e.SetClass("cls")
	e.SetInner("kept")

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)

	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Header()["Content-Type"], headerContentTypeJavaScript)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
	is.Equal(strings.Contains(w.Body.String(), `setAttribute("title","\x3c/script>\x3cimg onerror=alert(1) src=x>");`), true)
	is.Equal(strings.Contains(w.Body.String(), `classList?.add("cls");`), true)
	is.Equal(strings.Contains(w.Body.String(), "kept"), false)
	is.Equal(jw.RequestCount(), 1)
}

func TestServeHTTP_TailScript_EndpointIsPerRequest(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)

	req = httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w = httptest.NewRecorder()
	jw.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNoContent)
}

func TestServeHTTP_TailScript_WriteError(t *testing.T) {
	is := newTestHelper(t)
	jw, _ := New()
	go jw.Serve()
	defer jw.Close()

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	item := &testUi{}
	rq.NewElement(item).SetClass("cls")

	req := httptest.NewRequest(http.MethodGet, "/jaws/.tail/"+rq.JawsKeyString(), nil)
	req.RemoteAddr = hr.RemoteAddr
	w := &errResponseWriter{writeErr: errors.New("write failed")}
	jw.ServeHTTP(w, req)

	is.Equal(w.writeCall > 0, true)
	is.Equal(w.Header()["Content-Type"], headerContentTypeJavaScript)
	is.Equal(w.Header()["Cache-Control"], headerCacheControlNoStore)
	is.Equal(jw.RequestCount(), 1)
}
