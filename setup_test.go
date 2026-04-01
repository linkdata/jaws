package jaws

import (
	"fmt"
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

type testSetupper struct {
}

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
	jw.Setup(mux.Handle, prefix, extraStyle, ss1, u, ts.JawsSetupFunc, []*staticserve.StaticServe{ss2, ss3})
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

func TestJaws_SetupPanics(t *testing.T) {
	defer func() {
		x := recover()
		if x != nil {
			if y := fmt.Sprint(x); strings.HasPrefix(y, "expected ") {
				return
			}
		}
		t.Error(x)
	}()
	jw, _ := New()
	defer jw.Close()
	mux := http.NewServeMux()
	_ = jw.Setup(mux.Handle, "", 1)
	t.Fail()
}
