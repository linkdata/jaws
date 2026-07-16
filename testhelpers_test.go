package jaws

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/wire"
)

// maybePanic panics if err is non-nil. Test-only helper.
func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

type testJaws struct {
	*Jaws
	testtmpl *template.Template
	log      bytes.Buffer
}

func newTestJaws() (tj *testJaws) {
	jw, err := New()
	if err != nil {
		panic(err)
	}
	tj = &testJaws{Jaws: jw}
	tj.Jaws.Logger = slog.New(slog.NewTextHandler(&tj.log, nil))
	tj.Jaws.MakeAuth = func(r *Request) Auth {
		return &DefaultAuth{logger: tj.Logger}
	}
	tj.testtmpl = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}{{.}}{{end}}`))
	_ = tj.AddTemplateLookuper(tj.testtmpl)

	tj.Jaws.updateTicker = time.NewTicker(time.Millisecond)
	go tj.Serve()
	return
}

func (tj *testJaws) newRequest(r *http.Request) (tr *TestRequest) {
	return NewTestRequest(tj.Jaws, r)
}

func newTestRequest(t *testing.T) (tr *testRequest) {
	tj := newTestJaws()
	if t != nil {
		t.Helper()
		t.Cleanup(tj.Close)
	}
	return newWrappedTestRequest(tj.Jaws, nil)
}

type testRequest struct {
	*TestRequest
	rw testRequestWriter
}

func newWrappedTestRequest(jw *Jaws, r *http.Request) *testRequest {
	tr := NewTestRequest(jw, r)
	if tr == nil {
		return nil
	}
	return &testRequest{
		TestRequest: tr,
		rw: testRequestWriter{
			rq:     tr.Request,
			Writer: tr.ResponseRecorder,
		},
	}
}

func (tr *testRequest) UI(ui UI, params ...any) error    { return tr.rw.UI(ui, params...) }
func (tr *testRequest) Initial() *http.Request           { return tr.rw.Initial() }
func (tr *testRequest) HeadHTML() error                  { return tr.rw.HeadHTML() }
func (tr *testRequest) TailHTML() error                  { return tr.rw.TailHTML() }
func (tr *testRequest) Session() *Session                { return tr.rw.Session() }
func (tr *testRequest) Get(key string) (value any)       { return tr.rw.Get(key) }
func (tr *testRequest) Set(key string, value any)        { tr.rw.Set(key, value) }
func (tr *testRequest) Register(u Updater, p ...any) Jid { return tr.rw.Register(u, p...) }
func (tr *testRequest) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return tr.rw.Template(outerHTMLTag, name, dot, params...)
}

type testRequestWriter struct {
	rq *Request
	io.Writer
}

type testRegisterUI struct{ Updater }

func (testRegisterUI) JawsRender(elem *Element, w io.Writer, params []any) error { return nil }
func (u testRegisterUI) JawsUpdate(elem *Element)                                { u.Updater.JawsUpdate(elem) }

func (rw testRequestWriter) UI(ui UI, params ...any) error {
	return rw.rq.NewElement(ui).JawsRender(rw, params)
}

func (rw testRequestWriter) Write(p []byte) (n int, err error) {
	rw.rq.MarkWritten()
	return rw.Writer.Write(p)
}

func (rw testRequestWriter) Request() *Request {
	return rw.rq
}

func (rw testRequestWriter) Initial() *http.Request {
	return rw.rq.Initial()
}

func (rw testRequestWriter) HeadHTML() error {
	return rw.rq.HeadHTML(rw)
}

func (rw testRequestWriter) TailHTML() error {
	return rw.rq.TailHTML(rw)
}

func (rw testRequestWriter) Session() *Session {
	return rw.rq.Session()
}

func (rw testRequestWriter) Get(key string) (value any) {
	return rw.rq.Get(key)
}

func (rw testRequestWriter) Set(key string, value any) {
	rw.rq.Set(key, value)
}

func (rw testRequestWriter) Register(updater Updater, params ...any) Jid {
	elem := rw.rq.NewElement(testRegisterUI{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	elem.Freeze()
	return elem.Jid()
}

func (rq *Request) Writer(w io.Writer) testRequestWriter {
	return testRequestWriter{rq: rq, Writer: w}
}

type testHandler struct {
	*Jaws
	Template testTemplateUI
}

func (h testHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(w, nil))
}

func (jw *Jaws) Handler(outerHTMLTag, name string, dot any) http.Handler {
	return testHandler{Jaws: jw, Template: testTemplateUI{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}}
}

type testWith struct {
	*Element
	testRequestWriter
	Dot  any
	Auth Auth
}

type testTemplateUI struct {
	OuterHTMLTag string
	Name         string
	Dot          any
}

func (t testTemplateUI) String() string {
	return fmt.Sprintf("{%q, %q, %s}", t.OuterHTMLTag, t.Name, tag.TagString(t.Dot))
}

func (t testTemplateUI) execute(elem *Element, w io.Writer, tmpl *template.Template) (err error) {
	var auth Auth = &DefaultAuth{logger: elem.Jaws.Logger}
	if f := elem.Request.Jaws.MakeAuth; f != nil {
		auth = f(elem.Request)
	}
	err = tmpl.Execute(w, testWith{
		Element:           elem,
		testRequestWriter: testRequestWriter{rq: elem.Request, Writer: w},
		Dot:               t.Dot,
		Auth:              auth,
	})
	return
}

func writeTestTemplateWrapperStart(elem *Element, w io.Writer, outerHTMLTag string, attrs []string) (err error) {
	b := elem.Jid().AppendStartTagAttr(nil, outerHTMLTag)
	for _, attr := range attrs {
		if attr != "" {
			b = append(b, ' ')
			b = append(b, attr...)
		}
	}
	b = append(b, '>')
	_, err = w.Write(b)
	return
}

func (t testTemplateUI) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
	doWrap := t.OuterHTMLTag != ""
	var expandedTags []any
	if expandedTags, err = tag.TagExpand(elem.Request, t.Dot); err == nil {
		elem.Request.TagExpanded(elem, expandedTags)
		tags, handlers, attrs := ParseParams(params)
		elem.Tag(tags...)
		elem.AddHandlers(handlers...)
		err = fmt.Errorf("missing template %q", t.Name)
		if tmpl := elem.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			err = nil
			if doWrap {
				err = writeTestTemplateWrapperStart(elem, w, t.OuterHTMLTag, attrs)
			}
			if err == nil {
				if err = t.execute(elem, w, tmpl); err == nil {
					if doWrap {
						_, err = io.WriteString(w, "</"+t.OuterHTMLTag+">")
					}
				}
			}
		}
	}
	return
}

func (t testTemplateUI) JawsUpdate(elem *Element) {
	if t.OuterHTMLTag != "" {
		if tmpl := elem.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			var sb strings.Builder
			if err := t.execute(elem, &sb, tmpl); err != nil {
				elem.Request.MustLog(err)
			} else {
				elem.SetInner(template.HTML(sb.String())) // #nosec G203
			}
		} else {
			elem.Request.MustLog(fmt.Errorf("missing template %q", t.Name))
		}
	}
}

func (t testTemplateUI) JawsClick(elem *Element, click Click) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(ClickHandler); ok {
		err = h.JawsClick(elem, click)
	}
	return
}

func (t testTemplateUI) JawsContextMenu(elem *Element, click Click) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(ContextMenuHandler); ok {
		err = h.JawsContextMenu(elem, click)
	}
	return
}

func (t testTemplateUI) JawsInput(elem *Element, value string) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(InputHandler); ok {
		err = h.JawsInput(elem, value)
	}
	return
}

func (rw testRequestWriter) Template(outerHTMLTag, name string, dot any, params ...any) error {
	return rw.UI(testTemplateUI{OuterHTMLTag: outerHTMLTag, Name: name, Dot: dot}, params...)
}

type testDivWidget struct {
	inner template.HTML
}

func (u testDivWidget) JawsRender(elem *Element, w io.Writer, params []any) error {
	elem.ApplyParams(params)
	return htmlio.WriteHTMLInner(w, elem.Jid(), "div", "", u.inner)
}

func (testDivWidget) JawsUpdate(elem *Element) {}

type testTextInputWidget struct {
	setter   testStringSetter
	tagValue any
	last     string
}

type testStringSetter interface {
	JawsGet(elem *Element) string
	JawsSet(elem *Element, value string) error
}

func newTestTextInputWidget(s testStringSetter) *testTextInputWidget {
	return &testTextInputWidget{setter: s}
}

func (u *testTextInputWidget) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if u.tagValue, getterAttrs, err = elem.ApplyGetter(u.setter); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		v := u.setter.JawsGet(elem)
		u.last = v
		err = htmlio.WriteHTMLInput(w, elem.Jid(), "text", v, attrs)
	}
	return
}

func (u *testTextInputWidget) JawsUpdate(elem *Element) {
	if v := u.setter.JawsGet(elem); v != u.last {
		u.last = v
		elem.SetValue(v)
	}
}

func (u *testTextInputWidget) JawsInput(elem *Element, value string) (err error) {
	// Mirrors the canonical lib/ui applyDirty semantics: mark dirty unless the
	// set was a no-op (ErrValueUnchanged), propagate a real error, and only
	// record the new value as last-sent on success.
	if setErr := u.setter.JawsSet(elem, value); !errors.Is(setErr, ErrValueUnchanged) {
		err = setErr
		elem.Dirty(u.tagValue)
		if err == nil {
			u.last = value
		}
	}
	return
}

func nextBroadcast(t *testing.T, jw *Jaws) wire.Message {
	t.Helper()
	select {
	case msg := <-jw.bcastCh:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for broadcast")
		return wire.Message{}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

func printGoroutineOrigins(t *testing.T) {
	t.Helper()
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	buf = buf[:n]

	lines := bytes.Split(buf, []byte("\n"))
	re := regexp.MustCompile(`\t(.*?):(\d+) \+0x`)
	counts := make(map[string]int)

	for _, line := range lines {
		m := re.FindSubmatch(line)
		if len(m) == 3 {
			loc := fmt.Sprintf("%s:%s", m[1], m[2])
			counts[loc]++
		}
	}

	type pair struct {
		loc   string
		count int
	}
	var items []pair
	for k, v := range counts {
		if v > 1 {
			items = append(items, pair{k, v})
		}
	}

	slices.SortFunc(items, func(a, b pair) int {
		return cmp.Compare(b.count, a.count)
	})

	for _, item := range items {
		t.Logf("%-50s %4d goroutines\n", item.loc, item.count)
	}
}

type testHelper struct {
	*time.Timer
	*testing.T
}

func newTestHelper(t *testing.T) (th *testHelper) {
	seconds := 3
	if deadlock.Debug {
		seconds *= 10
	}
	th = &testHelper{
		T:     t,
		Timer: time.NewTimer(time.Second * time.Duration(seconds)),
	}
	t.Cleanup(th.Cleanup)
	return
}

func (th *testHelper) Cleanup() {
	th.Timer.Stop()
}

func (th *testHelper) Equal(got, want any) {
	if !testEqual(got, want) {
		th.Helper()
		th.Errorf("\n got %T(%#v)\nwant %T(%#v)\n", got, got, want, want)
	}
}

func (th *testHelper) True(a bool) {
	if !a {
		th.Helper()
		th.Error("not true")
	}
}

func (th *testHelper) NoErr(err error) {
	if err != nil {
		th.Helper()
		th.Error(err)
	}
}

func (th *testHelper) Timeout() {
	th.Helper()
	printGoroutineOrigins(th.T)
	th.Fatalf("timeout")
}

func Test_testHelper(t *testing.T) {
	mustEqual := func(a, b any) {
		if !testEqual(a, b) {
			t.Helper()
			t.Errorf("%#v != %#v", a, b)
		}
	}

	mustNotEqual := func(a, b any) {
		if testEqual(a, b) {
			t.Helper()
			t.Errorf("%#v == %#v", a, b)
		}
	}

	mustEqual(1, 1)
	mustEqual(nil, nil)
	mustEqual(nil, (*testHelper)(nil))
	mustNotEqual(1, nil)
	mustNotEqual(nil, 1)
	mustNotEqual((*testing.T)(nil), 1)
	mustNotEqual(1, 2)
	mustNotEqual((*testing.T)(nil), (*testHelper)(nil))
	mustNotEqual(int(1), int32(1))
}

func testNil(object any) (bool, reflect.Type) {
	if object == nil {
		return true, nil
	}
	value := reflect.ValueOf(object)
	kind := value.Kind()
	return kind >= reflect.Chan && kind <= reflect.Slice && value.IsNil(), value.Type()
}

func testEqual(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	aIsNil, aType := testNil(a)
	bIsNil, bType := testNil(b)
	if !(aIsNil && bIsNil) {
		return false
	}
	return aType == nil || bType == nil || (aType == bType)
}

type testSetter[T comparable] struct {
	mu        deadlock.Mutex
	val       T
	err       error
	setCount  int
	getCount  int
	setCalled chan struct{}
	getCalled chan struct{}
}

func newTestSetter[T comparable](value T) *testSetter[T] {
	return &testSetter[T]{
		val:       value,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
}

func (ts *testSetter[T]) Get() (value T) {
	ts.mu.Lock()
	value = ts.val
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) Set(value T) {
	ts.mu.Lock()
	ts.val = value
	ts.mu.Unlock()
}

func (ts *testSetter[T]) Err() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.err
}

func (ts *testSetter[T]) SetErr(err error) {
	ts.mu.Lock()
	ts.err = err
	ts.mu.Unlock()
}

func (ts *testSetter[T]) SetCount() (n int) {
	ts.mu.Lock()
	n = ts.setCount
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) GetCount() (n int) {
	ts.mu.Lock()
	n = ts.getCount
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) JawsGet(elem *Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[T]) JawsSet(elem *Element, value T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == value {
			err = ErrValueUnchanged
		}
		ts.val = value
	}
	return
}

func (ts *testSetter[string]) JawsGetString(elem *Element) (value string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[any]) JawsGetAny(elem *Element) (value any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

func (ts *testSetter[any]) JawsSetAny(elem *Element, value any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == value {
			err = ErrValueUnchanged
		}
		ts.val = value
	}
	return
}

func (ts *testSetter[T]) JawsGetHTML(elem *Element) (value T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	value = ts.val
	return
}

// closeRequestInBubble shuts a test request and its Jaws down from inside a
// synctest bubble, then waits for every bubbled goroutine (the request process
// loop, its event caller, and the Jaws Serve loop) to exit. synctest.Test
// requires the bubble to be free of live goroutines before it returns, so the
// usual t.Cleanup-based teardown (which runs outside the bubble) is too late.
func closeRequestInBubble(rq *testRequest) {
	rq.Close()
	rq.Jaws.Close()
	synctest.Wait()
}

// TestRequest is a request harness intended for the jaws package's own tests.
// The importable harness for other packages lives in
// github.com/linkdata/jaws/jawstest.
type TestRequest struct {
	*Request
	*requestHarness
}

type requestHarness struct {
	Req *Request
	*httptest.ResponseRecorder
	ReadyCh     chan struct{}
	DoneCh      chan struct{}
	InCh        chan wire.WsMsg
	OutCh       chan wire.WsMsg
	BcastCh     chan wire.Message
	ExpectPanic bool
	Panicked    bool
	PanicVal    any
}

func newRequestHarness(jw *Jaws, r *http.Request) (rh *requestHarness) {
	if r == nil {
		r = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	rr := httptest.NewRecorder()
	rr.Body = &bytes.Buffer{}
	rq := jw.NewRequest(r)
	if rq == nil || jw.UseRequest(rq.JawsKey, r) != rq {
		return nil
	}
	rh = &requestHarness{
		Req:              rq,
		ResponseRecorder: rr,
	}
	// The subscribe/process/recycle dance lives in jw.TestServe. The onPanic
	// callback adds this harness's panic-expectation support: an expected panic is
	// captured for inspection while an unexpected one is re-raised exactly as
	// before. It reads ExpectPanic lazily so tests may set it after construction.
	rh.InCh, rh.OutCh, rh.BcastCh, rh.ReadyCh, rh.DoneCh = jw.TestServe(rq, func(recovered any) {
		if recovered == nil {
			return
		}
		if rh.ExpectPanic {
			rh.PanicVal = recovered
			rh.Panicked = true
			return
		}
		panic(recovered)
	})
	return
}

// Close stops the test request's processing loop.
func (rh *requestHarness) Close() {
	close(rh.InCh)
}

// BodyString returns the recorded response body with surrounding whitespace removed.
func (rh *requestHarness) BodyString() string {
	return strings.TrimSpace(rh.Body.String())
}

// BodyHTML returns the recorded response body as trusted HTML.
func (rh *requestHarness) BodyHTML() template.HTML {
	return template.HTML(rh.BodyString()) /* #nosec G203 */
}

// NewTestRequest creates a TestRequest for use when testing.
// Passing nil for r creates a GET / request with no body.
func NewTestRequest(jw *Jaws, r *http.Request) (tr *TestRequest) {
	rh := newRequestHarness(jw, r)
	if rh != nil {
		tr = &TestRequest{
			Request:        rh.Req,
			requestHarness: rh,
		}
	}
	return
}

func TestErrEventUnhandled_Error(t *testing.T) {
	if got := ErrEventUnhandled.Error(); got != "event unhandled" {
		t.Fatalf("ErrEventUnhandled.Error() = %q, want %q", got, "event unhandled")
	}
}

func TestNewRequestHarness_ReturnsNilOnClaimFailure(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	jw.reqPool.New = func() any {
		rq := (&Request{
			Jaws:   jw,
			tagMap: make(map[any][]*Element),
		}).clearLocked()
		rq.claimed.Store(true)
		return rq
	}

	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	if rh := newRequestHarness(jw, hr); rh != nil {
		t.Fatal("expected nil harness when claim fails")
	}
}

func TestNewTestRequest_PanicsWhenJawsClosed(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Close()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when the Jaws instance is closed")
		}
	}()
	NewTestRequest(jw, nil)
}

func TestTestServe_TimesOutWhenServeNotRunning(t *testing.T) {
	// Without a running Serve/ServeWithTimeout loop nothing drains subCh, so the
	// subscription rendezvous in TestServe can neither complete nor see Done, and
	// it must panic after its 5s timeout. Run in a synctest bubble so that timeout
	// elapses in fake time rather than stalling the test for five real seconds.
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		defer jw.Close()
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		defer func() {
			s, ok := recover().(string)
			if !ok || !strings.Contains(s, "timed out subscribing") {
				t.Fatalf("expected timeout panic, got %v", s)
			}
		}()
		jw.TestServe(rq, func(any) {})
		t.Fatal("expected TestServe to panic")
	})
}

func TestTestServe_PanicsWhenClosedAfterSubscribing(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	subCh := make(chan subscription)
	jw.subCh = subCh
	readyCh := make(chan struct{})
	go func() {
		close(readyCh)
		<-subCh
		jw.Close()
	}()
	<-readyCh

	defer func() {
		s, ok := recover().(string)
		if !ok || !strings.Contains(s, "Jaws instance is closed") {
			t.Fatalf("expected closed panic, got %v", s)
		}
	}()
	jw.TestServe(rq, func(any) {})
	t.Fatal("expected TestServe to panic")
}

func TestTestServe_TimesOutAfterSubscribing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		jw, err := New()
		if err != nil {
			t.Fatal(err)
		}
		defer jw.Close()
		rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		subCh := make(chan subscription)
		jw.subCh = subCh
		readyCh := make(chan struct{})
		go func() {
			close(readyCh)
			<-subCh
		}()
		<-readyCh

		defer func() {
			s, ok := recover().(string)
			if !ok || !strings.Contains(s, "timed out subscribing") {
				t.Fatalf("expected timeout panic, got %v", s)
			}
		}()
		jw.TestServe(rq, func(any) {})
		t.Fatal("expected TestServe to panic")
	})
}

func TestNewTestRequest_SuccessPathAndClose(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(jw.Close)

	go jw.Serve()

	tr := NewTestRequest(jw, nil)
	if tr == nil {
		t.Fatal("expected test request")
	}

	if tr.Initial() == nil {
		t.Fatal("expected initial request")
	}

	tr.Close()
	select {
	case <-tr.DoneCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for test request shutdown")
	}
}
