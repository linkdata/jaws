package core

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/linkdata/jaws/staticserve"
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
	if x := jw.FaviconURL(); x != path.Join(prefix, ss1.Name) {
		t.Error(x)
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
