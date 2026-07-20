package jaws

import (
	"net/http"
	"net/http/httptest"
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

func TestJaws_SetupDoesNotPrefixExternalOriginURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const rawURL = "https://cdn.example.test"
	if err = jw.Setup(nil, "/static", rawURL); err != nil {
		t.Fatal(err)
	}

	if got := jw.headPrefix; strings.Contains(got, rawURL+"/static") {
		t.Fatalf("Setup prefixed external origin URL: %q", got)
	}
	if got := jw.headPrefix; !strings.Contains(got, `href="`+rawURL+`"`) {
		t.Fatalf("Setup head HTML = %q, want unmodified external URL %q", got, rawURL)
	}
}

// TestJaws_SetupDoesNotPrefixProtocolRelativeURL guards the Host=="" clause of
// makeAbsPath specifically: a protocol-relative URL carries a host but no scheme,
// so a scheme-only check (such as url.URL.IsAbs) would still prefix and corrupt it.
func TestJaws_SetupDoesNotPrefixProtocolRelativeURL(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()

	const rawURL = "//cdn.example.test"
	if err = jw.Setup(nil, "/static", rawURL); err != nil {
		t.Fatal(err)
	}

	if got := jw.headPrefix; strings.Contains(got, rawURL+"/static") {
		t.Fatalf("Setup prefixed protocol-relative URL: %q", got)
	}
	if got := jw.headPrefix; !strings.Contains(got, `href="`+rawURL+`"`) {
		t.Fatalf("Setup head HTML = %q, want unmodified protocol-relative URL %q", got, rawURL)
	}
}

func TestJaws_SetupEmptyPrefix(t *testing.T) {
	ss := staticserve.Must("favicon.png", []byte("Hello"))

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	mux := http.NewServeMux()
	if err = jw.Setup(mux.Handle, "", ss); err != nil {
		t.Fatal(err)
	}

	headURL := jw.FaviconURL()
	if want := "/" + ss.Name; headURL != want {
		t.Fatalf("favicon URL = %q, want %q", headURL, want)
	}
	if !strings.Contains(jw.headPrefix, `href="`+headURL+`"`) {
		t.Fatalf("head HTML %q does not contain favicon URL %q", jw.headPrefix, headURL)
	}

	pageURL, err := url.Parse("http://example.test/account/view")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := url.Parse(headURL)
	if err != nil {
		t.Fatal(err)
	}
	resolved := pageURL.ResolveReference(ref)
	if resolved.Path != headURL {
		t.Fatalf("favicon URL %q resolves from nested page to %q", headURL, resolved.Path)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, resolved.String(), nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %q = %d, want %d", resolved.Path, rr.Code, http.StatusOK)
	}
	if got := rr.Body.String(); got != "Hello" {
		t.Fatalf("GET %q body = %q, want %q", resolved.Path, got, "Hello")
	}
}

func TestJaws_SetupStaticServeEscapesName(t *testing.T) {
	ss := staticserve.Must(`favicon:scheme {asset}#query?percent%\file.png`, []byte("Hello"))

	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	mux := http.NewServeMux()
	if err = jw.Setup(mux.Handle, "", ss); err != nil {
		t.Fatal(err)
	}

	headURL := jw.FaviconURL()
	if !strings.HasPrefix(headURL, "/favicon:scheme") {
		t.Fatalf("favicon URL is not slash-rooted: %q", headURL)
	}
	for _, escaped := range []string{"%20", "%7Basset%7D", "%23", "%3F", "%25", "%5C"} {
		if !strings.Contains(headURL, escaped) {
			t.Errorf("favicon URL %q does not contain %q", headURL, escaped)
		}
	}

	u, err := url.Parse(headURL)
	if err != nil {
		t.Fatal(err)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		t.Fatalf("favicon URL parsed with query %q and fragment %q", u.RawQuery, u.Fragment)
	}

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, headURL, nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("GET %q = %d, want %d", headURL, rr.Code, http.StatusOK)
	}

	wildcardURL := strings.Replace(headURL, "%7Basset%7D", "other", 1)
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, wildcardURL, nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET wildcard candidate %q = %d, want %d", wildcardURL, rr.Code, http.StatusNotFound)
	}
}

func TestJaws_SetupEmptyPrefixKeepsGenericRelativeURLs(t *testing.T) {
	urlExtra, err := url.Parse("url.css")
	if err != nil {
		t.Fatal(err)
	}
	setupURL, err := url.Parse("setup.css")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		extra any
		want  string
	}{
		{name: "string", extra: "string.css", want: "string.css"},
		{name: "URL", extra: urlExtra, want: "url.css"},
		{name: "SetupFunc", extra: SetupFunc(func(_ *Jaws, _ HandleFunc, _ string) (urls []*url.URL, err error) {
			urls = append(urls, setupURL)
			return
		}), want: "setup.css"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()
			if err = jw.Setup(nil, "", tc.extra); err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(jw.headPrefix, `href="`+tc.want+`"`) {
				t.Fatalf("head HTML %q does not contain relative URL %q", jw.headPrefix, tc.want)
			}
			if strings.Contains(jw.headPrefix, `href="/`+tc.want+`"`) {
				t.Fatalf("head HTML %q slash-rooted relative URL %q", jw.headPrefix, tc.want)
			}
		})
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
