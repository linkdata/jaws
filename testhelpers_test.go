package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"text/template/parse"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/htmlio"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/wire"
)

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
		return DefaultAuth{}
	}
	tj.testtmpl = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}" {{$.Attrs}}>{{.}}</div>{{end}}`))
	tj.AddTemplateLookuper(tj.testtmpl)

	tj.Jaws.updateTicker = time.NewTicker(time.Millisecond)
	go tj.Serve()
	return
}

func (tj *testJaws) newRequest(hr *http.Request) (tr *TestRequest) {
	return NewTestRequest(tj.Jaws, hr)
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

func newWrappedTestRequest(jw *Jaws, hr *http.Request) *testRequest {
	tr := NewTestRequest(jw, hr)
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
func (tr *testRequest) Get(key string) (val any)         { return tr.rw.Get(key) }
func (tr *testRequest) Set(key string, val any)          { tr.rw.Set(key, val) }
func (tr *testRequest) Register(u Updater, p ...any) Jid { return tr.rw.Register(u, p...) }
func (tr *testRequest) Template(name string, dot any, params ...any) error {
	return tr.rw.Template(name, dot, params...)
}

type testRequestWriter struct {
	rq *Request
	io.Writer
}

type testRegisterUI struct{ Updater }

func (testRegisterUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (u testRegisterUI) JawsUpdate(e *Element)                     { u.Updater.JawsUpdate(e) }

func (rw testRequestWriter) UI(ui UI, params ...any) error {
	return rw.rq.NewElement(ui).JawsRender(rw, params)
}

func (rw testRequestWriter) Write(p []byte) (n int, err error) {
	rw.rq.Rendering.Store(true)
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

func (rw testRequestWriter) Get(key string) (val any) {
	return rw.rq.Get(key)
}

func (rw testRequestWriter) Set(key string, val any) {
	rw.rq.Set(key, val)
}

func (rw testRequestWriter) Register(updater Updater, params ...any) Jid {
	elem := rw.rq.NewElement(testRegisterUI{Updater: updater})
	elem.Tag(updater)
	elem.ApplyParams(params)
	updater.JawsUpdate(elem)
	return elem.Jid()
}

func (rq *Request) Writer(w io.Writer) testRequestWriter {
	return testRequestWriter{rq: rq, Writer: w}
}

type testHandler struct {
	*Jaws
	Template testTemplateUI
}

func (h testHandler) ServeHTTP(wr http.ResponseWriter, r *http.Request) {
	_ = h.Log(h.NewRequest(r).NewElement(h.Template).JawsRender(wr, nil))
}

func (jw *Jaws) Handler(name string, dot any) http.Handler {
	return testHandler{Jaws: jw, Template: testTemplateUI{Name: name, Dot: dot}}
}

type testWith struct {
	*Element
	testRequestWriter
	Dot   any
	Attrs template.HTMLAttr
	Auth  Auth
}

type testTemplateUI struct {
	Name string
	Dot  any
}

func (t testTemplateUI) String() string {
	return fmt.Sprintf("{%q, %s}", t.Name, tag.TagString(t.Dot))
}

func findJidOrJsOrHTMLNode(node parse.Node) (found bool) {
	switch node := node.(type) {
	case *parse.TextNode:
		if node != nil {
			found = found || bytes.Contains(node.Text, []byte("</html>"))
		}
	case *parse.ListNode:
		if node != nil {
			for _, n := range node.Nodes {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.ActionNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
		}
	case *parse.WithNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(&node.BranchNode)
		}
	case *parse.BranchNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
			found = found || findJidOrJsOrHTMLNode(node.List)
			found = found || findJidOrJsOrHTMLNode(node.ElseList)
		}
	case *parse.IfNode:
		if node != nil {
			found = findJidOrJsOrHTMLNode(node.Pipe)
			found = found || findJidOrJsOrHTMLNode(node.List)
			found = found || findJidOrJsOrHTMLNode(node.ElseList)
		}
	case *parse.PipeNode:
		if node != nil {
			for _, n := range node.Cmds {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.CommandNode:
		if node != nil {
			for _, n := range node.Args {
				found = found || findJidOrJsOrHTMLNode(n)
			}
		}
	case *parse.VariableNode:
		if node != nil {
			for _, s := range node.Ident {
				found = found || (s == "Jid") || (s == "JsFunc") || (s == "JsVar")
			}
		}
	}
	return
}

func (t testTemplateUI) JawsRender(e *Element, wr io.Writer, params []any) (err error) {
	var expandedtags []any
	if expandedtags, err = tag.TagExpand(e.Request, t.Dot); err == nil {
		e.Request.TagExpanded(e, expandedtags)
		tags, handlers, attrs := ParseParams(params)
		e.Tag(tags...)
		e.AddHandlers(handlers...)
		attrstr := template.HTMLAttr(strings.Join(attrs, " ")) // #nosec G203
		var auth Auth = DefaultAuth{}
		if f := e.Request.Jaws.MakeAuth; f != nil {
			auth = f(e.Request)
		}
		err = fmt.Errorf("missing template %q", t.Name)
		if tmpl := e.Request.Jaws.LookupTemplate(t.Name); tmpl != nil {
			err = tmpl.Execute(wr, testWith{
				Element:           e,
				testRequestWriter: testRequestWriter{rq: e.Request, Writer: wr},
				Dot:               t.Dot,
				Attrs:             attrstr,
				Auth:              auth,
			})
			if deadlock.Debug && e.Jaws.Logger != nil {
				if !findJidOrJsOrHTMLNode(tmpl.Tree.Root) {
					e.Jaws.Logger.Warn("jaws: template has no Jid reference", "template", t.Name)
				}
			}
		}
	}
	return
}

func (t testTemplateUI) JawsUpdate(e *Element) {
	if dot, ok := t.Dot.(Updater); ok {
		dot.JawsUpdate(e)
	}
}

func (t testTemplateUI) JawsClick(e *Element, click Click) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(ClickHandler); ok {
		err = h.JawsClick(e, click)
	}
	return
}

func (t testTemplateUI) JawsContextMenu(e *Element, click Click) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(ContextMenuHandler); ok {
		err = h.JawsContextMenu(e, click)
	}
	return
}

func (t testTemplateUI) JawsInput(e *Element, val string) (err error) {
	err = ErrEventUnhandled
	if h, ok := t.Dot.(InputHandler); ok {
		err = h.JawsInput(e, val)
	}
	return
}

func (rw testRequestWriter) Template(name string, dot any, params ...any) error {
	return rw.UI(testTemplateUI{Name: name, Dot: dot}, params...)
}

type testDivWidget struct {
	inner template.HTML
}

func (u testDivWidget) JawsRender(e *Element, w io.Writer, params []any) error {
	e.ApplyParams(params)
	return htmlio.WriteHTMLInner(w, e.Jid(), "div", "", u.inner)
}

func (testDivWidget) JawsUpdate(*Element) {}

type testTextInputWidget struct {
	setter   testStringSetter
	tagValue any
	last     string
}

type testStringSetter interface {
	JawsGet(*Element) string
	JawsSet(*Element, string) error
}

func newTestTextInputWidget(s testStringSetter) *testTextInputWidget {
	return &testTextInputWidget{setter: s}
}

func (u *testTextInputWidget) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if u.tagValue, getterAttrs, err = e.ApplyGetter(u.setter); err == nil {
		attrs := append(e.ApplyParams(params), getterAttrs...)
		v := u.setter.JawsGet(e)
		u.last = v
		err = htmlio.WriteHTMLInput(w, e.Jid(), "text", v, attrs)
	}
	return
}

func (u *testTextInputWidget) JawsUpdate(e *Element) {
	if v := u.setter.JawsGet(e); v != u.last {
		u.last = v
		e.SetValue(v)
	}
}

func (u *testTextInputWidget) JawsInput(e *Element, val string) (err error) {
	if changed, setErr := e.maybeDirty(u.tagValue, u.setter.JawsSet(e, val)); setErr != nil {
		err = setErr
	} else {
		if changed {
			u.last = val
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

	sort.Slice(items, func(i, j int) bool {
		return items[i].count > items[j].count
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

func newTestSetter[T comparable](val T) *testSetter[T] {
	return &testSetter[T]{
		val:       val,
		setCalled: make(chan struct{}),
		getCalled: make(chan struct{}),
	}
}

func (ts *testSetter[T]) Get() (val T) {
	ts.mu.Lock()
	val = ts.val
	ts.mu.Unlock()
	return
}

func (ts *testSetter[T]) Set(val T) {
	ts.mu.Lock()
	ts.val = val
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

func (ts *testSetter[T]) JawsGet(*Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[T]) JawsSet(_ *Element, val T) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[string]) JawsGetString(*Element) (val string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsGetAny(*Element) (val any) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}

func (ts *testSetter[any]) JawsSetAny(_ *Element, val any) (err error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.setCount++
	if ts.setCount == 1 {
		close(ts.setCalled)
	}
	if err = ts.err; err == nil {
		if ts.val == val {
			err = ErrValueUnchanged
		}
		ts.val = val
	}
	return
}

func (ts *testSetter[T]) JawsGetHTML(*Element) (val T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.getCount++
	if ts.getCount == 1 {
		close(ts.getCalled)
	}
	val = ts.val
	return
}
