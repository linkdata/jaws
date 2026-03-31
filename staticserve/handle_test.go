package staticserve_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/linkdata/jaws/internal/testutil"
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
	filepaths := testutil.AssetFilepaths(t, assetsFS, "assets")
	expected := testutil.ExpectedStaticAssets(t, assetsFS, "assets", "/", filepaths...)

	mux := http.NewServeMux()
	uris, err := staticserve.HandleFS(assetsFS, mux.Handle, "assets", filepaths...)
	if err != nil {
		t.Fatal(err)
	}
	if len(uris) != len(expected) {
		t.Fatal(len(uris))
	}

	for i, exp := range expected {
		if uris[i] != exp.URI {
			t.Errorf("%q: expected uri %q, got %q", exp.Filepath, exp.URI, uris[i])
		}

		rq := httptest.NewRequest(http.MethodGet, exp.URI, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		res := rr.Result()
		if sc := res.StatusCode; sc != http.StatusOK {
			t.Errorf("%q plain: expected status %d, got %d", exp.Filepath, http.StatusOK, sc)
		}
		if cc := res.Header.Get("Cache-Control"); cc != staticserve.HeaderCacheControl[0] {
			t.Errorf("%q plain: expected cache-control %q, got %q", exp.Filepath, staticserve.HeaderCacheControl[0], cc)
		}
		if vary := res.Header.Get("Vary"); vary != staticserve.HeaderVary[0] {
			t.Errorf("%q plain: expected vary %q, got %q", exp.Filepath, staticserve.HeaderVary[0], vary)
		}
		if ce := res.Header.Get("Content-Encoding"); ce != "" {
			t.Errorf("%q plain: expected empty content-encoding, got %q", exp.Filepath, ce)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.ContentType {
			t.Errorf("%q plain: expected content type %q, got %q", exp.Filepath, exp.ContentType, ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.Plain) {
			t.Errorf("%q plain: body mismatch", exp.Filepath)
		}
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}

		rq = httptest.NewRequest(http.MethodGet, exp.URI, nil)
		rq.Header.Set("Accept-Encoding", "gzip")
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		res = rr.Result()
		if sc := res.StatusCode; sc != http.StatusOK {
			t.Errorf("%q gzip: expected status %d, got %d", exp.Filepath, http.StatusOK, sc)
		}
		if cc := res.Header.Get("Cache-Control"); cc != staticserve.HeaderCacheControl[0] {
			t.Errorf("%q gzip: expected cache-control %q, got %q", exp.Filepath, staticserve.HeaderCacheControl[0], cc)
		}
		if vary := res.Header.Get("Vary"); vary != staticserve.HeaderVary[0] {
			t.Errorf("%q gzip: expected vary %q, got %q", exp.Filepath, staticserve.HeaderVary[0], vary)
		}
		if ce := res.Header.Get("Content-Encoding"); ce != "gzip" {
			t.Errorf("%q gzip: expected content-encoding %q, got %q", exp.Filepath, "gzip", ce)
		}
		if cl := res.Header.Get("Content-Length"); cl != strconv.Itoa(len(exp.Gz)) {
			t.Errorf("%q gzip: expected content-length %d, got %q", exp.Filepath, len(exp.Gz), cl)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.ContentType {
			t.Errorf("%q gzip: expected content type %q, got %q", exp.Filepath, exp.ContentType, ct)
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.Gz) {
			t.Errorf("%q gzip: body mismatch", exp.Filepath)
		}
		if err := res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
