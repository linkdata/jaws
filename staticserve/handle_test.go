package staticserve_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linkdata/jaws/staticserve"
)

func TestHandle_Pattern(t *testing.T) {
	var gotPattern string
	uri, err := staticserve.Handle("file.txt", []byte("abc"), func(pattern string, _ http.Handler) {
		gotPattern = pattern
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "GET " + uri; gotPattern != want {
		t.Fatalf("expected pattern %q, got %q", want, gotPattern)
	}
}

func TestHandleFS(t *testing.T) {
	filepaths := assetFilepaths(t, assetsFS, "assets")
	expected := expectedStaticAssets(t, assetsFS, "assets", "/", filepaths...)

	mux := http.NewServeMux()
	uris, err := staticserve.HandleFS(assetsFS, mux.Handle, "assets", filepaths...)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != len(expected) {
		t.Fatal(len(uris))
	}

	for i, exp := range expected {
		if uris[i] != exp.uri {
			t.Errorf("%q: expected uri %q, got %q", exp.filepath, exp.uri, uris[i])
		}

		rq := httptest.NewRequest(http.MethodGet, exp.uri, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		res := rr.Result()
		if sc := res.StatusCode; sc != http.StatusOK {
			t.Errorf("%q plain: expected status %d, got %d", exp.filepath, http.StatusOK, sc)
		}
		if cc := res.Header.Get("Cache-Control"); cc != staticserve.HeaderCacheControl[0] {
			t.Errorf("%q plain: expected cache-control %q, got %q", exp.filepath, staticserve.HeaderCacheControl[0], cc)
		}
		if vary := res.Header.Get("Vary"); vary != staticserve.HeaderVary[0] {
			t.Errorf("%q plain: expected vary %q, got %q", exp.filepath, staticserve.HeaderVary[0], vary)
		}
		if ce := res.Header.Get("Content-Encoding"); ce != "" {
			t.Errorf("%q plain: expected empty content-encoding, got %q", exp.filepath, ce)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.contentType {
			t.Errorf("%q plain: expected content type %q, got %q", exp.filepath, exp.contentType, ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.plain) {
			t.Errorf("%q plain: body mismatch", exp.filepath)
		}
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}

		rq = httptest.NewRequest(http.MethodGet, exp.uri, nil)
		rq.Header.Set("Accept-Encoding", "gzip")
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		res = rr.Result()
		if sc := res.StatusCode; sc != http.StatusOK {
			t.Errorf("%q gzip: expected status %d, got %d", exp.filepath, http.StatusOK, sc)
		}
		if cc := res.Header.Get("Cache-Control"); cc != staticserve.HeaderCacheControl[0] {
			t.Errorf("%q gzip: expected cache-control %q, got %q", exp.filepath, staticserve.HeaderCacheControl[0], cc)
		}
		if vary := res.Header.Get("Vary"); vary != staticserve.HeaderVary[0] {
			t.Errorf("%q gzip: expected vary %q, got %q", exp.filepath, staticserve.HeaderVary[0], vary)
		}
		if ce := res.Header.Get("Content-Encoding"); ce != "gzip" {
			t.Errorf("%q gzip: expected content-encoding %q, got %q", exp.filepath, "gzip", ce)
		}
		if cl := res.Header.Get("Content-Length"); cl != strconv.Itoa(len(exp.gz)) {
			t.Errorf("%q gzip: expected content-length %d, got %q", exp.filepath, len(exp.gz), cl)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.contentType {
			t.Errorf("%q gzip: expected content type %q, got %q", exp.filepath, exp.contentType, ct)
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.gz) {
			t.Errorf("%q gzip: body mismatch", exp.filepath)
		}
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
