package ui

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
)

type testLoggerBarrier struct {
	done chan struct{}
}

func (*testLoggerBarrier) Error() string { return "test logger barrier" }

func testLoggedError(args []any) (err error) {
	for i := 0; i+1 < len(args); i += 2 {
		if args[i] == "err" {
			err, _ = args[i+1].(error)
			return
		}
	}
	return
}

func testCompleteLoggerBarrier(err error) bool {
	if barrier, ok := err.(*testLoggerBarrier); ok {
		close(barrier.done)
		return true
	}
	return false
}

func testSyncLogger(t *testing.T, jw *jaws.Jaws) {
	t.Helper()
	barrier := &testLoggerBarrier{done: make(chan struct{})}
	_ = jw.Log(barrier)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case <-barrier.done:
	case <-timer.C:
		t.Fatal("timed out waiting for asynchronous logger")
	}
}

type testErrorLog struct {
	mu     sync.Mutex
	errors []error
}

func (l *testErrorLog) record(args []any) {
	err := testLoggedError(args)
	if err == nil || testCompleteLoggerBarrier(err) {
		return
	}
	l.mu.Lock()
	l.errors = append(l.errors, err)
	l.mu.Unlock()
}

func (l *testErrorLog) snapshot() []error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]error(nil), l.errors...)
}

func (l *testErrorLog) sync(t *testing.T, jw *jaws.Jaws) []error {
	t.Helper()
	testSyncLogger(t, jw)
	return l.snapshot()
}

func mustMatch(t *testing.T, pattern, got string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.MatchString(got) {
		t.Fatalf("pattern %q did not match %q", pattern, got)
	}
}

func newCoreRequest(t *testing.T) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	return newConfiguredCoreRequest(t, nil)
}

// newConfiguredCoreRequest runs configure against the fresh [jaws.Jaws] before the
// [jaws.Request] exists, which the Jaws documentation requires of every exported
// configuration field: they are ordinary fields, so setting one after Requests are
// created is an unsynchronized write. Tests that need a Logger use this rather than
// assigning jw.Logger to a Jaws that already has a Request.
func newConfiguredCoreRequest(t *testing.T, configure func(*jaws.Jaws)) (*jaws.Jaws, *jaws.Request) {
	t.Helper()
	jw, err := jaws.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)
	if configure != nil {
		configure(jw)
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("nil request")
	}
	return jw, rq
}

// withLogger returns a configure func for [newConfiguredCoreRequest] that installs logger.
func withLogger(logger *templateLogger) func(*jaws.Jaws) {
	return func(jw *jaws.Jaws) { jw.Logger = logger }
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

func requireContainerState(t *testing.T, elem *jaws.Element) *containerState {
	t.Helper()
	st, ok := jaws.ElementState(elem).(*containerState)
	if !ok || st == nil {
		t.Fatalf("element %v state = %T, want *containerState", elem.Jid(), jaws.ElementState(elem))
	}
	return st
}

func containerElements(t *testing.T, elem *jaws.Element) (contents []*jaws.Element) {
	t.Helper()
	st := requireContainerState(t, elem)
	st.mu.Lock()
	contents = append(contents, st.contents...)
	st.mu.Unlock()
	return
}

type testHTMLGetter string

func (g testHTMLGetter) JawsGetHTML(elem *jaws.Element) template.HTML {
	return template.HTML(g)
}

type testSetter[T comparable] struct {
	mu       sync.Mutex
	v        T
	err      error
	setCount int
}

func newTestSetter[T comparable](value T) *testSetter[T] {
	return &testSetter[T]{v: value}
}

func (ts *testSetter[T]) JawsGet(elem *jaws.Element) T {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.v
}

func (ts *testSetter[T]) JawsSet(elem *jaws.Element, value T) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.err != nil {
		return ts.err
	}
	if ts.v == value {
		return jaws.ErrValueUnchanged
	}
	ts.v = value
	ts.setCount++
	return nil
}

func (ts *testSetter[T]) Set(value T) {
	ts.mu.Lock()
	ts.v = value
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

func (tc *testContainer) JawsContains(elem *jaws.Element) []jaws.UI {
	return tc.contents
}
