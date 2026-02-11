package ui

import (
	"errors"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws/core"
)

var jidPattern = regexp.MustCompile(`Jid\.[0-9]+`)

func mustMatch(t *testing.T, pattern, got string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.MatchString(got) {
		t.Fatalf("pattern %q did not match %q", pattern, got)
	}
}

func newRequest(t *testing.T) (*core.Jaws, *core.Request) {
	t.Helper()
	jw, err := core.New()
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

func renderUI(t *testing.T, rq *core.Request, ui core.UI, params ...any) (*core.Element, string) {
	t.Helper()
	elem := rq.NewElement(ui)
	var sb strings.Builder
	if err := elem.JawsRender(&sb, params); err != nil {
		t.Fatal(err)
	}
	return elem, sb.String()
}

type testHTMLGetter string

func (g testHTMLGetter) JawsGetHTML(*core.Element) template.HTML {
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

func (ts *testSetter[T]) JawsGet(*core.Element) T {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.v
}

func (ts *testSetter[T]) JawsSet(_ *core.Element, v T) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.err != nil {
		return ts.err
	}
	if ts.v == v {
		return core.ErrValueUnchanged
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
	contents []core.UI
}

func (tc *testContainer) JawsContains(*core.Element) []core.UI {
	return tc.contents
}

type errorUI struct {
	err error
}

func (ui errorUI) JawsRender(*core.Element, io.Writer, []any) error {
	if ui.err != nil {
		return ui.err
	}
	return errors.New("errorUI")
}

func (errorUI) JawsUpdate(*core.Element) {}

func waitUntil(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !fn() {
		if time.Now().After(deadline) {
			t.Fatal("timeout")
		}
		time.Sleep(time.Millisecond)
	}
}
