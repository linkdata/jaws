package jawsboot_test

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/internal/testutil"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/staticserve"
	"github.com/linkdata/jaws/ui"
)

//go:embed assets
var testAssetsFS embed.FS

func TestJawsBoot_Setup(t *testing.T) {
	const prefix = "/static"
	expected := testutil.ExpectedStaticAssets(t, testAssetsFS, "assets/static", prefix)
	mux := http.NewServeMux()

	jw, _ := jaws.New()
	defer jw.Close()

	err := jw.Setup(mux.Handle, prefix, jawsboot.Setup, "/other/foobar.js")
	if err != nil {
		t.Fatal(err)
	}

	rq := jw.NewRequest(nil)
	var sb strings.Builder
	ui.RequestWriter{Request: rq, Writer: &sb}.HeadHTML()
	txt := sb.String()
	if !strings.Contains(txt, rq.JawsKeyString()) {
		t.Error(txt)
	}
	for _, exp := range expected {
		if !strings.Contains(txt, `"`+exp.URI+`"`) {
			t.Errorf("expected head html to include %q", exp.URI)
		}
	}
	if !strings.Contains(txt, "\"/other/foobar.js\"") {
		t.Error(txt)
	}

	for _, exp := range expected {
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
		if ct := res.Header.Get("Content-Type"); ct != exp.SS.ContentType {
			t.Errorf("%q plain: expected content type %q, got %q", exp.Filepath, exp.SS.ContentType, ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.Plain) {
			t.Errorf("%q plain: body mismatch", exp.Filepath)
		}
		if err = res.Body.Close(); err != nil {
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
		if cl := res.Header.Get("Content-Length"); cl != strconv.Itoa(len(exp.SS.Gz)) {
			t.Errorf("%q gzip: expected content-length %d, got %q", exp.Filepath, len(exp.SS.Gz), cl)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.SS.ContentType {
			t.Errorf("%q gzip: expected content type %q, got %q", exp.Filepath, exp.SS.ContentType, ct)
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.SS.Gz) {
			t.Errorf("%q gzip: body mismatch", exp.Filepath)
		}
		if unpacked := testutil.ReadGzip(t, b); !bytes.Equal(unpacked, exp.Plain) {
			t.Errorf("%q gzip: unpacked body mismatch", exp.Filepath)
		}
		if err = res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}

	for _, mapURI := range []string{
		path.Join(prefix, "bootstrap.bundle.min.js.map"),
		path.Join(prefix, "bootstrap.min.css.map"),
	} {
		rq := httptest.NewRequest(http.MethodGet, mapURI, nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, rq)
		res := rr.Result()
		if sc := res.StatusCode; sc != http.StatusNotFound {
			t.Errorf("%q: expected status %d, got %d", mapURI, http.StatusNotFound, sc)
		}
		if ct := res.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
			t.Errorf("%q: expected content type %q, got %q", mapURI, "text/plain; charset=utf-8", ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, []byte("404 page not found\n")) {
			t.Errorf("%q: unexpected body", mapURI)
		}
		if err = res.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
