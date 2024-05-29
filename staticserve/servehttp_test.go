package staticserve_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

const someText = `The quick brown fox jumps over the lazy dog.`

func Test_ServeHTTP_Raw(t *testing.T) {
	ss, err := staticserve.New("test.txt", []byte(someText))
	if err != nil {
		t.Fatal(err)
	}
	rq := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	ss.ServeHTTP(rr, rq)
	if sc := rr.Result().StatusCode; sc != http.StatusOK {
		t.Error(sc)
	}
	b, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte(someText)) {
		t.Error(string(b))
	}
}

func Test_ServeHTTP_GZip(t *testing.T) {
	ss, err := staticserve.New("test.txt", []byte(someText))
	if err != nil {
		t.Fatal(err)
	}
	rq := httptest.NewRequest(http.MethodGet, "/", nil)
	rq.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()
	ss.ServeHTTP(rr, rq)
	res := rr.Result()
	if sc := res.StatusCode; sc != http.StatusOK {
		t.Error(sc)
	}
	if ce := res.Header.Get("Content-Encoding"); ce != "gzip" {
		t.Error(res.Header)
	}
	b, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, ss.Gz) {
		t.Error("data mismatch")
	}
}

func Test_ServeHTTP_Errors(t *testing.T) {
	ss := &staticserve.StaticServe{
		Gz: []byte{0},
	}
	rq := httptest.NewRequest(http.MethodPut, "/", nil)
	rr := httptest.NewRecorder()
	ss.ServeHTTP(rr, rq)
	if sc := rr.Result().StatusCode; sc != http.StatusMethodNotAllowed {
		t.Error(sc)
	}

	rq = httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	ss.ServeHTTP(rr, rq)
	if sc := rr.Result().StatusCode; sc != http.StatusInternalServerError {
		t.Error(sc)
	}
}
