package jawsboot_test

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/jawsboot"
	"github.com/linkdata/jaws/lib/ui"
	"github.com/linkdata/staticserve"
)

//go:embed assets
var testAssetsFS embed.FS

// Asset files are already tracked by git. Keep these tests focused on serving,
// headers and integration behavior; do not add stored-hash provenance tests for
// files whose contents and history are in the repository.

func TestJawsBoot_Setup(t *testing.T) {
	const prefix = "/static"
	expected := expectedStaticAssets(t, testAssetsFS, "assets/static", prefix)
	mux := http.NewServeMux()

	jw, _ := jaws.New()
	defer jw.Close()

	err := jw.Setup(mux.Handle, prefix, jawsboot.Setup, "/other/foobar.js")
	if err != nil {
		t.Fatal(err)
	}

	rq := jw.NewRequest(nil)
	var sb strings.Builder
	if err = (ui.RequestWriter{Request: rq, Writer: &sb}).HeadHTML(); err != nil {
		t.Fatal(err)
	}
	txt := sb.String()
	if !strings.Contains(txt, rq.JawsKeyString()) {
		t.Error(txt)
	}
	for _, exp := range expected {
		if !strings.Contains(txt, `"`+exp.uri+`"`) {
			t.Errorf("expected head html to include %q", exp.uri)
		}
	}
	if !strings.Contains(txt, "\"/other/foobar.js\"") {
		t.Error(txt)
	}

	for _, exp := range expected {
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
		if ct := res.Header.Get("Content-Type"); ct != exp.ss.ContentType {
			t.Errorf("%q plain: expected content type %q, got %q", exp.filepath, exp.ss.ContentType, ct)
		}
		b, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.plain) {
			t.Errorf("%q plain: body mismatch", exp.filepath)
		}
		if err = res.Body.Close(); err != nil {
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
		if cl := res.Header.Get("Content-Length"); cl != strconv.Itoa(len(exp.ss.Gz)) {
			t.Errorf("%q gzip: expected content-length %d, got %q", exp.filepath, len(exp.ss.Gz), cl)
		}
		if ct := res.Header.Get("Content-Type"); ct != exp.ss.ContentType {
			t.Errorf("%q gzip: expected content type %q, got %q", exp.filepath, exp.ss.ContentType, ct)
		}
		b, err = io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(b, exp.ss.Gz) {
			t.Errorf("%q gzip: body mismatch", exp.filepath)
		}
		if unpacked := readGzip(t, b); !bytes.Equal(unpacked, exp.plain) {
			t.Errorf("%q gzip: unpacked body mismatch", exp.filepath)
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

func TestJawsBoot_SetupNilHandleFuncGeneratesHead(t *testing.T) {
	const prefix = "/static"
	expected := expectedStaticAssets(t, testAssetsFS, "assets/static", prefix)

	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Setup with nil HandleFunc panicked: %v", recovered)
		}
	}()

	if err := jw.Setup(nil, prefix, jawsboot.Setup); err != nil {
		t.Fatal(err)
	}

	rq := jw.NewRequest(nil)
	var sb strings.Builder
	if err := (ui.RequestWriter{Request: rq, Writer: &sb}).HeadHTML(); err != nil {
		t.Fatal(err)
	}
	head := sb.String()
	for _, exp := range expected {
		if !strings.Contains(head, `"`+exp.uri+`"`) {
			t.Errorf("expected head html to include %q", exp.uri)
		}
	}
}

// TestJawsBoot_SetupPrefixVariants verifies that for any prefix form (absolute,
// relative, parent-relative or empty) every asset URL emitted into the head HTML
// resolves to a registered handler.
func TestJawsBoot_SetupPrefixVariants(t *testing.T) {
	assets := expectedStaticAssets(t, testAssetsFS, "assets/static", "")
	for _, prefix := range []string{"/static", "static", "../static", ""} {
		t.Run("prefix="+strconv.Quote(prefix), func(t *testing.T) {
			mux := http.NewServeMux()
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()
			if err := jw.Setup(mux.Handle, prefix, jawsboot.Setup); err != nil {
				t.Fatal(err)
			}

			rq := jw.NewRequest(nil)
			var sb strings.Builder
			if err := (ui.RequestWriter{Request: rq, Writer: &sb}).HeadHTML(); err != nil {
				t.Fatal(err)
			}
			head := sb.String()

			for _, exp := range assets {
				wantURI := path.Join("/", prefix, exp.ss.Name)
				if !strings.Contains(head, `"`+wantURI+`"`) {
					t.Errorf("head html missing %q", wantURI)
				}
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, wantURI, nil))
				if rr.Code != http.StatusOK {
					t.Errorf("GET %q (prefix %q) = %d, want 200 (head URL must match a registered handler)", wantURI, prefix, rr.Code)
				}
			}

			for _, name := range []string{"bootstrap.bundle.min.js.map", "bootstrap.min.css.map"} {
				mapURI := path.Join("/", prefix, name)
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, mapURI, nil))
				if rr.Code != http.StatusNotFound {
					t.Errorf("GET %q (prefix %q) = %d, want 404 (sourcemap probe must be 404)", mapURI, prefix, rr.Code)
				}
			}
		})
	}
}

func TestJawsBoot_SetupLiteralBracePrefixes(t *testing.T) {
	assets := expectedStaticAssets(t, testAssetsFS, "assets/static", "")
	for _, tc := range []struct {
		name          string
		prefix        string
		outsidePrefix string
	}{
		{name: "absolute partial segment", prefix: "/static{assets}"},
		{name: "relative partial segment", prefix: "static{assets}"},
		{name: "complete wildcard segment", prefix: "/{assets}", outsidePrefix: "/outside"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const fallbackStatus = http.StatusTeapot
			mux := http.NewServeMux()
			mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(fallbackStatus)
			})
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(jw.Close)

			var (
				urls       []*url.URL
				setupErr   error
				panicValue any
			)
			func() {
				defer func() { panicValue = recover() }()
				urls, setupErr = jawsboot.Setup(jw, mux.Handle, tc.prefix)
			}()
			if panicValue != nil {
				t.Fatalf("Setup(%q) panicked: %v", tc.prefix, panicValue)
			}
			if setupErr != nil {
				t.Fatal(setupErr)
			}
			if len(urls) == 0 {
				t.Fatal("Setup returned no URLs")
			}
			if got, want := len(urls), len(assets); got != want {
				t.Errorf("Setup returned %d asset URLs, want %d", got, want)
			}
			returnedURLs := make(map[string]bool, len(urls))
			for _, u := range urls {
				returnedURLs[u.String()] = true
			}

			for _, exp := range assets {
				assetPath := staticserve.EnsurePrefixSlash(path.Join(tc.prefix, exp.ss.Name))
				assetURL := (&url.URL{Path: assetPath}).String()
				if !returnedURLs[assetURL] {
					t.Errorf("Setup(%q) did not return expected asset URL %q", tc.prefix, assetURL)
				}
				delete(returnedURLs, assetURL)
				r := httptest.NewRequest(http.MethodGet, assetURL, nil)
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, r)
				if rr.Code != http.StatusOK {
					t.Errorf("GET %q = %d, want 200", assetURL, rr.Code)
				}
				if _, pattern := mux.Handler(r); pattern != staticserve.NormalizeGET(assetURL) {
					t.Errorf("GET %q matched pattern %q, want literal pattern %q",
						assetURL, pattern, staticserve.NormalizeGET(assetURL))
				}

				if tc.outsidePrefix != "" {
					outsideURI := path.Join(tc.outsidePrefix, exp.ss.Name)
					outsideRequest := httptest.NewRequest(http.MethodGet, outsideURI, nil)
					outsideRecorder := httptest.NewRecorder()
					mux.ServeHTTP(outsideRecorder, outsideRequest)
					if outsideRecorder.Code != fallbackStatus {
						t.Errorf("GET %q = %d, want fallback status %d",
							outsideURI, outsideRecorder.Code, fallbackStatus)
					}
				}
			}
			for unexpectedURL := range returnedURLs {
				t.Errorf("Setup(%q) returned unexpected asset URL %q", tc.prefix, unexpectedURL)
			}

			for _, name := range []string{"bootstrap.bundle.min.js.map", "bootstrap.min.css.map"} {
				mapPath := staticserve.EnsurePrefixSlash(path.Join(tc.prefix, name))
				mapURL := (&url.URL{Path: mapPath}).String()
				r := httptest.NewRequest(http.MethodGet, mapURL, nil)
				if _, pattern := mux.Handler(r); pattern != staticserve.NormalizeGET(mapURL) {
					t.Errorf("GET %q matched pattern %q, want literal 404 pattern %q",
						mapURL, pattern, staticserve.NormalizeGET(mapURL))
				}
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, r)
				if rr.Code != http.StatusNotFound {
					t.Errorf("GET %q = %d, want 404", mapURL, rr.Code)
				}

				if tc.outsidePrefix != "" {
					outsideURI := path.Join(tc.outsidePrefix, name)
					outsideRequest := httptest.NewRequest(http.MethodGet, outsideURI, nil)
					outsideRecorder := httptest.NewRecorder()
					mux.ServeHTTP(outsideRecorder, outsideRequest)
					if outsideRecorder.Code != fallbackStatus {
						t.Errorf("GET %q = %d, want fallback status %d",
							outsideURI, outsideRecorder.Code, fallbackStatus)
					}
				}
			}
		})
	}
}

// TestJawsBoot_SetupReturnedURLs pins jawsboot.Setup's exported (urls, err) contract
// directly, independently of jaws.Setup's wrapping: every returned URL is absolute
// and resolves to a handler registered via the supplied HandleFunc, for absolute,
// relative, parent-relative and empty prefixes.
func TestJawsBoot_SetupReturnedURLs(t *testing.T) {
	for _, prefix := range []string{"/static", "static", "../static", ""} {
		t.Run("prefix="+strconv.Quote(prefix), func(t *testing.T) {
			jw, err := jaws.New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()

			mux := http.NewServeMux()
			registered := map[string]bool{}
			handleFn := func(pattern string, handler http.Handler) {
				registered[pattern] = true
				mux.Handle(pattern, handler)
			}

			urls, err := jawsboot.Setup(jw, handleFn, prefix)
			if err != nil {
				t.Fatal(err)
			}
			if len(urls) == 0 {
				t.Fatal("Setup returned no URLs")
			}
			for _, u := range urls {
				if !path.IsAbs(u.Path) {
					t.Errorf("returned URL %q is not absolute", u.String())
				}
				if u.Path != path.Clean(u.Path) {
					t.Errorf("returned URL path %q is not clean", u.Path)
				}
				if !registered[staticserve.NormalizeGET(u.String())] {
					t.Errorf("returned URL %q has no matching registered handler", u.String())
				}
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, u.String(), nil))
				if rr.Code != http.StatusOK {
					t.Errorf("GET returned URL %q = %d, want 200", u.String(), rr.Code)
				}
			}

			for _, name := range []string{"bootstrap.bundle.min.js.map", "bootstrap.min.css.map"} {
				mapURI := path.Join("/", prefix, name)
				if !registered[staticserve.NormalizeGET(mapURI)] {
					t.Errorf("source-map path %q has no matching registered handler", mapURI)
				}
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, mapURI, nil))
				if rr.Code != http.StatusNotFound {
					t.Errorf("GET source-map path %q = %d, want 404", mapURI, rr.Code)
				}
			}
		})
	}
}
