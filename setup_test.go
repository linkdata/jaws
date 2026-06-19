package jaws

import (
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/linkdata/staticserve"
)

type testMux struct {
	m map[string]http.Handler
}

func (tm *testMux) Handle(uri string, handler http.Handler) {
	if tm.m == nil {
		tm.m = make(map[string]http.Handler)
	}
	tm.m[uri] = handler
}

type testSetupper struct{}

func (ts *testSetupper) JawsSetupFunc(jw *Jaws, handleFn HandleFunc, prefix string) (urls []*url.URL, err error) {
	ss, _ := staticserve.New("foo.txt", []byte("foo"))
	u, _ := url.Parse(ss.Name)
	urls = append(urls, u)
	handleFn(path.Join(prefix, ss.Name), ss)
	return
}

func TestJaws_Setup(t *testing.T) {
	const prefix = "/static"
	const extraStyle = "someExtraStyle.css"
	ss1 := staticserve.Must("favicon.png", []byte("Hello"))
	ss2 := staticserve.Must("1.txt", []byte("Hello"))
	ss3 := staticserve.Must("2.txt", []byte("Hello"))
	u, _ := url.Parse("relative")
	ts := &testSetupper{}

	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	if err := jw.Setup(mux.Handle, prefix, extraStyle, ss1, u, ts.JawsSetupFunc, []*staticserve.StaticServe{ss2, ss3}); err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 4 {
		t.Log(len(mux.m))
		t.Error(mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss1.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss2.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	if _, ok := mux.m["GET "+path.Join(prefix, ss3.Name)]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
	var barePatterns []string
	for pattern := range mux.m {
		if !strings.HasPrefix(pattern, "GET ") {
			barePatterns = append(barePatterns, pattern)
		}
	}
	if len(barePatterns) != 1 {
		t.Fatalf("expected 1 bare path pattern, got %d: %#v", len(barePatterns), mux.m)
	}
	if got := barePatterns[0]; !strings.HasPrefix(got, prefix+"/foo.") || !strings.HasSuffix(got, ".txt") {
		t.Errorf("unexpected setupfunc pattern: %q", got)
	}
	if x := jw.FaviconURL(); x != path.Join(prefix, ss1.Name) {
		t.Error(x)
	}
}

func TestJaws_SetupURLExtraCanBeReusedWithDifferentPrefixes(t *testing.T) {
	u, err := url.Parse("app.js")
	if err != nil {
		t.Fatal(err)
	}

	jw1, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw1.Close()
	if err = jw1.Setup(nil, "/one", u); err != nil {
		t.Fatal(err)
	}

	jw2, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw2.Close()
	if err = jw2.Setup(nil, "/two", u); err != nil {
		t.Fatal(err)
	}

	if got := jw2.headPrefix; !strings.Contains(got, `/two/app.js`) {
		t.Fatalf("second Setup head = %q, want reused URL extra under /two", got)
	}
	if got := u.String(); got != "app.js" {
		t.Fatalf("Setup mutated URL extra: got %q, want %q", got, "app.js")
	}
}

func TestJaws_SetupRelativePrefixYieldsAbsoluteURL(t *testing.T) {
	// A relative, non-empty prefix must still produce a head URL that matches the
	// always-absolute handler pattern; a relative URL would resolve against the
	// current page and 404 on any non-root page.
	ss := staticserve.Must("favicon.png", []byte("Hello"))

	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	if err := jw.Setup(mux.Handle, "static", ss); err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	var pattern string
	for p := range mux.m {
		pattern = p
	}
	headURL := jw.FaviconURL()
	if !strings.HasPrefix(headURL, "/static/favicon.") {
		t.Errorf("head URL is not absolute: %q", headURL)
	}
	if want := "GET " + headURL; pattern != want {
		t.Errorf("handler pattern %q does not match head URL: want %q", pattern, want)
	}
}

func TestJaws_SetupEmptyPrefix(t *testing.T) {
	ss := staticserve.Must("favicon.png", []byte("Hello"))

	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	_ = jw.Setup(mux.Handle, "", ss)

	if got := jw.FaviconURL(); got != ss.Name {
		t.Errorf("unexpected favicon URL: %q", got)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	for pattern := range mux.m {
		if want := "GET /" + ss.Name; pattern != want {
			t.Errorf("expected pattern %q, got %q", want, pattern)
		}
	}
}

func TestJaws_SetupKeepsMethodPattern(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	err := jw.Setup(mux.Handle, "", SetupFunc(func(_ *Jaws, handleFn HandleFunc, _ string) (urls []*url.URL, err error) {
		handleFn("POST\t/custom", http.NotFoundHandler())
		return
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	if _, ok := mux.m["POST\t/custom"]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
}

func TestJaws_SetupSetupFuncBarePathIsUnchanged(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := &testMux{}
	err := jw.Setup(mux.Handle, "", SetupFunc(func(_ *Jaws, handleFn HandleFunc, _ string) (urls []*url.URL, err error) {
		handleFn("custom", http.NotFoundHandler())
		return
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(mux.m) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(mux.m))
	}
	if _, ok := mux.m["custom"]; !ok {
		t.Errorf("registered patterns: %#v", mux.m)
	}
}

func TestJaws_SetupRejectsUnknownExtra(t *testing.T) {
	jw, _ := New()
	defer jw.Close()
	mux := http.NewServeMux()
	err := jw.Setup(mux.Handle, "", 1)
	if err == nil {
		t.Fatal("expected an error for an unsupported extra type")
	}
	if !strings.Contains(err.Error(), "not int") {
		t.Errorf("unexpected error: %v", err)
	}
}
