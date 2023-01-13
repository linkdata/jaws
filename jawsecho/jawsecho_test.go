package jawsecho_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsecho"
	"github.com/matryer/is"
)

func TestJawsEcho_GetJavascript(t *testing.T) {
	is := is.New(t)
	jw := jaws.New()
	go jw.Serve()
	defer jw.Close()

	e := echo.New()
	jawsecho.Setup(e, jw)

	req := httptest.NewRequest("", jaws.JavascriptPath, nil)
	w := httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(jaws.JavascriptText))

	req = httptest.NewRequest("", jaws.JavascriptPath, nil)
	req.Header.Add(echo.HeaderAcceptEncoding, "gzip")
	w = httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)
	is.Equal(w.Body.Len(), len(jaws.JavascriptGZip))
}

func TestJawsEcho_GetPing(t *testing.T) {
	is := is.New(t)
	jw := jaws.New()
	go jw.Serve()
	defer jw.Close()

	e := echo.New()
	jawsecho.Setup(e, jw)

	req := httptest.NewRequest("", "/jaws/ping", nil)
	w := httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusOK)

	jw.Close()

	req = httptest.NewRequest("", "/jaws/ping", nil)
	w = httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusServiceUnavailable)
}

func TestJawsEcho_GetKey(t *testing.T) {
	is := is.New(t)
	jw := jaws.New()
	go jw.Serve()
	defer jw.Close()

	e := echo.New()
	jawsecho.Setup(e, jw)

	req := httptest.NewRequest("", "/jaws/notaproperkey", nil)
	w := httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)

	req = httptest.NewRequest("", "/jaws/12345678", nil)
	w = httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusNotFound)

	rq := jw.NewRequest(context.Background(), req)
	req = httptest.NewRequest("", "/jaws/"+rq.JawsKeyString(), nil)
	w = httptest.NewRecorder()
	e.Server.Handler.ServeHTTP(w, req)
	is.Equal(w.Code, http.StatusUpgradeRequired)
}
