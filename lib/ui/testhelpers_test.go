package ui

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws"
)

func mustMatch(t *testing.T, pattern, got string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.MatchString(got) {
		t.Fatalf("pattern %q did not match %q", pattern, got)
	}
}

func newCoreRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("nil request")
	}
	return jw, rq
}

func newCoreSessionBoundRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	if sess := jw.NewSession(rr, hr); sess == nil {
		t.Fatal("expected session")
	}
	rq := jw.NewRequest(hr)
	if rq == nil {
		t.Fatal("expected request")
	}
	return jw, rq
}

func renderUI(t *testing.T, rq *jaws.Request, ui jaws.UI, params ...any) (*jaws.Element, string) {
	t.Helper()
	elem := rq.NewElement(ui)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, params); err != nil {
		t.Fatal(err)
	}
	return elem, sb.String()
}

type testHTMLGetter string

func (g testHTMLGetter) JawsGetHTML(*jaws.Element) template.HTML {
	return template.HTML(g)
}

type testSetter[T comparable] struct {
	mu       sync.Mutex
	v        T
	err      error
	setCount int
}

func newTestSetter[T comparable](v T) *testSetter[T] {
	return &testSetter[T]{v: v}
}

func (ts *testSetter[T]) JawsGet(*jaws.Element) T {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.v
}

func (ts *testSetter[T]) JawsSet(_ *jaws.Element, v T) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.err != nil {
		return ts.err
	}
	if ts.v == v {
		return jaws.ErrValueUnchanged
	}
	ts.v = v
	ts.setCount++
	return nil
}

func (ts *testSetter[T]) Set(v T) {
	ts.mu.Lock()
	ts.v = v
	ts.mu.Unlock()
}

func (ts *testSetter[T]) Get() T {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.v
}

func (ts *testSetter[T]) SetErr(err error) {
	ts.mu.Lock()
	ts.err = err
	ts.mu.Unlock()
}

type testContainer struct {
	contents []jaws.UI
}

func (tc *testContainer) JawsContains(*jaws.Element) []jaws.UI {
	return tc.contents
}
