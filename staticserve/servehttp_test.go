package staticserve_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func Test_ServeHTTP_JavaScriptContentType_FromGZipInput(t *testing.T) {
	js := []byte("console.log('jaws');")
	ssJS, err := staticserve.New("test.js", js)
	if err != nil {
		t.Fatal(err)
	}
	ss, err := staticserve.New("test.JS.gz", ssJS.Gz)
	if err != nil {
		t.Fatal(err)
	}

	testCases := []struct {
		AcceptEncoding string
		WantBody       []byte
		WantEncoding   string
	}{
		{WantBody: js},
		{AcceptEncoding: "gzip", WantBody: ss.Gz, WantEncoding: "gzip"},
	}

	for _, tc := range testCases {
		rq := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.AcceptEncoding != "" {
			rq.Header.Set("Accept-Encoding", tc.AcceptEncoding)
		}
		rr := httptest.NewRecorder()
		ss.ServeHTTP(rr, rq)
		res := rr.Result()
		if sc := res.StatusCode; sc != http.StatusOK {
			t.Fatalf("status code %d", sc)
		}
		if got := res.Header.Get("Content-Encoding"); got != tc.WantEncoding {
			t.Fatalf("expected content-encoding %q, got %q", tc.WantEncoding, got)
		}
		if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Fatalf("expected javascript content type, got %q", ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, tc.WantBody) {
			t.Fatal("body mismatch")
		}
		if err = res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
