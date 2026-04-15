package jaws

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/tag"
	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type testUi struct {
	renderCalled int32
	updateCalled int32
	getCalled    int32
	setCalled    int32
	initCalled   int32
	initError    error
	s            string
	renderFn     func(e *Element, w io.Writer, params []any) error
	updateFn     func(e *Element)
}

// JawsInit implements InitHandler.
func (tss *testUi) JawsInit(e *Element) (err error) {
	atomic.AddInt32(&tss.initCalled, 1)
	return tss.initError
}

var _ UI = (*testUi)(nil)
var _ InitHandler = (*testUi)(nil)

func (tss *testUi) JawsGet(e *Element) string {
	atomic.AddInt32(&tss.getCalled, 1)
	return tss.s
}

func (tss *testUi) JawsSet(e *Element, s string) error {
	atomic.AddInt32(&tss.setCalled, 1)
	tss.s = s
	return nil
}

func (tss *testUi) JawsRender(e *Element, w io.Writer, params []any) (err error) {
	e.Tag(tss)
	atomic.AddInt32(&tss.renderCalled, 1)
	if tss.renderFn != nil {
		err = tss.renderFn(e, w, params)
	}
	return
}

func (tss *testUi) JawsUpdate(e *Element) {
	atomic.AddInt32(&tss.updateCalled, 1)
	if tss.updateFn != nil {
		tss.updateFn(e)
	}
}

type testApplyGetterAll struct {
	initErr error
}

func (a testApplyGetterAll) JawsGetTag(tag.Context) any { return tag.Tag("tg") }
func (a testApplyGetterAll) JawsClick(*Element, Click) error {
	return ErrEventUnhandled
}
func (a testApplyGetterAll) JawsEvent(*Element, what.What, string) error {
	return ErrEventUnhandled
}
func (a testApplyGetterAll) JawsInit(*Element) error {
	return a.initErr
}

func TestElement_helpers(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{}
	e := rq.NewElement(tss)
	is.Equal(e.Jaws, rq.Jaws)
	is.Equal(e.Request, rq.Request)
	is.Equal(e.Session(), nil)
	e.Set("foo", "bar") // no session, so no effect
	is.Equal(e.Get("foo"), nil)
}

func TestElement_Tag(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{}
	e := rq.NewElement(tss)
	is.True(!e.HasTag(tag.Tag("zomg")))
	e.Tag(tag.Tag("zomg"))
	is.True(e.HasTag(tag.Tag("zomg")))
	s := e.String()
	if !strings.Contains(s, "zomg") {
		t.Error(s)
	}
}

func TestElement_Queued(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{
		updateFn: func(e *Element) {
			e.SetAttr("hidden", "")
			e.RemoveAttr("hidden")
			e.SetClass("bah")
			e.RemoveClass("bah")
			e.SetValue("foo")
			e.SetInner("meh")
			e.Append("<div></div>")
			e.Remove("some-id")
			e.Order([]jid.Jid{1, 2})
			replaceHTML := template.HTML(fmt.Sprintf("<div id=\"%s\"></div>", e.Jid().String()))
			e.Replace(replaceHTML)
			th.Equal(rq.wsQueue, []wire.WsMsg{
				{
					Data: "hidden\n",
					Jid:  e.jid,
					What: what.SAttr,
				},
				{
					Data: "hidden",
					Jid:  e.jid,
					What: what.RAttr,
				},
				{
					Data: "bah",
					Jid:  e.jid,
					What: what.SClass,
				},
				{
					Data: "bah",
					Jid:  e.jid,
					What: what.RClass,
				},
				{
					Data: "foo",
					Jid:  e.jid,
					What: what.Value,
				},
				{
					Data: "meh",
					Jid:  e.jid,
					What: what.Inner,
				},
				{
					Data: "<div></div>",
					Jid:  e.jid,
					What: what.Append,
				},
				{
					Data: "some-id",
					Jid:  e.jid,
					What: what.Remove,
				},
				{
					Data: fmt.Sprintf("%s %s", Jid(1).String(), Jid(2).String()),
					Jid:  e.jid,
					What: what.Order,
				},
				{
					Data: string(replaceHTML),
					Jid:  e.jid,
					What: what.Replace,
				},
			})
		},
	}

	pendingRq := rq.Jaws.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	testRequestWriter{rq: pendingRq, Writer: httptest.NewRecorder()}.UI(tss)

	rq.UI(tss)
	rq.Jaws.Dirty(tss)
	rq.Dirty(tss)
	for atomic.LoadInt32(&tss.updateCalled) < 1 {
		select {
		case <-th.C:
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	th.Equal(tss.renderCalled, int32(2))
}

func TestElement_ReplacePanicsOnMissingId(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	defer func() {
		if x := recover(); x == nil {
			is.Fail()
		}
	}()
	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)
	e.Replace(template.HTML("<div id=\"wrong\"></div>"))
	is.Fail()
}

func TestElement_ReplaceMessageTargetsElementHTML(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	tagValue := &testUi{}
	jid := rq.Register(tagValue)
	elem := rq.GetElementByJid(jid)
	if elem == nil {
		t.Fatal("missing element")
	}
	html := `<div id="` + jid.String() + `">replaced</div>`

	elem.Replace(template.HTML(html))
	// Element.Replace queues directly on the Request, so poke the process loop
	// once to ensure queued messages are flushed to OutCh in this harness.
	select {
	case rq.InCh <- wire.WsMsg{}:
	case <-time.After(time.Second):
		t.Fatal("timeout waking request process loop")
	}
	msg := nextOutboundMsg(t, rq)

	if msg.What != what.Replace {
		t.Fatalf("unexpected message type %v", msg.What)
	}
	if msg.Data != html {
		t.Fatalf("replace payload mismatch: got %q want %q", msg.Data, html)
	}
}

func TestElement_maybeDirty(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()
	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)

	changed, err := e.maybeDirty(e, nil)
	th.True(changed)
	th.NoErr(err)

	changed, err = e.maybeDirty(e, ErrValueUnchanged)
	th.Equal(changed, false)
	th.Equal(err, nil)

	changed, err = e.maybeDirty(e, tag.ErrNotComparable)
	th.Equal(changed, false)
	th.Equal(err, tag.ErrNotComparable)
}

func TestElement_RenderDebugAndDeletedBranches(t *testing.T) {
	NextJid = 0
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	tu := &testUi{renderFn: func(*Element, io.Writer, []any) error { return nil }}
	elem := rq.NewElement(tu)

	rq.mu.Lock()
	var sb strings.Builder
	elem.renderDebug(&sb)
	rq.mu.Unlock()

	elem.Tag(tag.Tag("a"), tag.Tag("b"))
	sb.Reset()
	elem.renderDebug(&sb)
	if !strings.Contains(sb.String(), ", ") {
		t.Fatal("expected comma-separated tags in debug output")
	}

	rq.Jaws.Debug = true
	sb.Reset()
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	rq.Jaws.Debug = false

	rq.DeleteElement(elem)
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	elem.JawsUpdate()
}

func TestElement_ApplyGetterDebugBranches(t *testing.T) {
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(&testUi{})

	if gotTag, err := elem.ApplyGetter(nil); gotTag != nil || err != nil {
		t.Fatalf("unexpected %v %v", gotTag, err)
	}

	ag := testApplyGetterAll{}
	gotTags, err := elem.ApplyGetter(ag)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !elem.HasTag(tag.Tag("tg")) {
		t.Fatalf("missing Tag('tg') in %#v", gotTags)
	}
	agErr := testApplyGetterAll{initErr: tag.ErrNotComparable}
	if _, err := elem.ApplyGetter(agErr); err != tag.ErrNotComparable {
		t.Fatalf("expected init err, got %v", err)
	}

	if deadlock.Debug {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic for non-comparable UI in debug mode")
			}
		}()
		rq.NewElement(testUnhashableUI{m: map[string]int{"x": 1}})
	}
}

type testClickHandler struct {
}

func (tch testClickHandler) JawsClick(e *Element, click Click) (err error) {
	return nil
}

var _ ClickHandler = testClickHandler{}

type testNonComparableClickHandler struct {
	names []string
}

func (tch testNonComparableClickHandler) JawsClick(e *Element, click Click) error {
	return nil
}

var _ ClickHandler = testNonComparableClickHandler{}

type testEventHandler struct{}

func (testEventHandler) JawsEvent(*Element, what.What, string) error {
	return nil
}

var _ EventHandler = testEventHandler{}

type testNonComparableEventHandler struct {
	names []string
}

func (testNonComparableEventHandler) JawsEvent(*Element, what.What, string) error {
	return nil
}

var _ EventHandler = testNonComparableEventHandler{}

type testContextMenuHandler struct{}

func (testContextMenuHandler) JawsContextMenu(*Element, Click) error {
	return nil
}

var _ ContextMenuHandler = testContextMenuHandler{}

type testNonComparableContextMenuHandler struct {
	names []string
}

func (testNonComparableContextMenuHandler) JawsContextMenu(*Element, Click) error {
	return nil
}

var _ ContextMenuHandler = testNonComparableContextMenuHandler{}

type testInitialHTMLAttrHandler struct {
	attr template.HTMLAttr
}

func (h testInitialHTMLAttrHandler) JawsInitialHTMLAttr(*Element) (s template.HTMLAttr) {
	s = h.attr
	return
}

var _ InitialHTMLAttrHandler = testInitialHTMLAttrHandler{}

type testStringSetterWithInitialHTMLAttr struct {
	s    string
	attr template.HTMLAttr
}

func (s *testStringSetterWithInitialHTMLAttr) JawsGet(*Element) (retv string) {
	retv = s.s
	return
}

func (s *testStringSetterWithInitialHTMLAttr) JawsSet(_ *Element, next string) (err error) {
	s.s = next
	return
}

func (s *testStringSetterWithInitialHTMLAttr) JawsInitialHTMLAttr(*Element) (retv template.HTMLAttr) {
	retv = s.attr
	return
}

type testClickAndInitialHTMLAttr struct {
	called *bool
	attr   template.HTMLAttr
}

func (h testClickAndInitialHTMLAttr) JawsClick(*Element, Click) error {
	if h.called != nil {
		*h.called = true
	}
	return nil
}

func (h testClickAndInitialHTMLAttr) JawsInitialHTMLAttr(*Element) (retv template.HTMLAttr) {
	retv = h.attr
	return
}

var _ ClickHandler = testClickAndInitialHTMLAttr{}
var _ InitialHTMLAttrHandler = testClickAndInitialHTMLAttr{}

type testUnhashableUI struct {
	m map[string]int
}

func (testUnhashableUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (testUnhashableUI) JawsUpdate(*Element)                         {}

func TestElement_ApplyGetter(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)

	var tch testClickHandler
	gotTag, err := e.ApplyGetter(tch)
	if gotTag != tch {
		t.Errorf("tag was %#v", gotTag)
	}
	if err != nil {
		t.Error(err)
	}
	is.Equal(len(e.handlers), 1)
	if !e.HasTag(tch) {
		t.Fatal("expected comparable click handler to be tagged")
	}
}

func TestElement_ApplyGetter_NonComparableHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(&testUi{s: "foo"})
	tch := testNonComparableClickHandler{names: []string{"name"}}
	if _, err := e.ApplyGetter(tch); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.Ui(), e, what.Click, "1 2 0 name"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
}

func TestElement_ApplyParams_NonComparableHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	tch := testNonComparableClickHandler{names: []string{"name"}}
	e.ApplyParams([]any{tch})
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.Ui(), e, what.Click, "1 2 0 name"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
}

func TestElement_ApplyParams_InitialHTMLAttrHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	attrs := e.ApplyParams([]any{
		"hidden",
		testInitialHTMLAttrHandler{attr: `data-attr="ok"`},
	})
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attr, got %d", len(attrs))
	}
	if attrs[0] != "hidden" {
		t.Fatalf("unexpected first attr %q", attrs[0])
	}
	if strings.Contains(string(attrs[0]), "data-attr") {
		t.Fatalf("unexpected initial HTML attr in ApplyParams output: %q", attrs[0])
	}
}

func TestElement_ApplyGetter_InitialHTMLAttrHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	setter := &testStringSetterWithInitialHTMLAttr{
		s:    "foo",
		attr: `data-from-getter="yes"`,
	}
	if err := rq.UI(newTestTextInputWidget(setter)); err != nil {
		t.Fatal(err)
	}
	got := rq.BodyString()
	if !strings.Contains(got, `data-from-getter="yes"`) {
		t.Fatalf("missing getter attr in %q", got)
	}
	if strings.Count(got, `data-from-getter="yes"`) != 1 {
		t.Fatalf("expected one getter attr in %q", got)
	}
	if len(rq.elems) != 1 {
		t.Fatalf("expected one element, got %d", len(rq.elems))
	}
	if len(rq.elems[0].handlers) != 0 {
		t.Fatalf("expected initial attr handler to be removed, got %d handlers", len(rq.elems[0].handlers))
	}
}

func TestElement_ApplyParams_RemovesOnlyInitialHTMLAttrHandlers(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	called := false
	h := testClickAndInitialHTMLAttr{
		called: &called,
		attr:   `data-attr="ok"`,
	}
	if _, err := e.ApplyGetter(h); err != nil {
		t.Fatal(err)
	}
	if len(e.handlers) != 2 {
		t.Fatalf("expected 2 handlers before ApplyParams, got %d", len(e.handlers))
	}
	attrs := e.ApplyParams(nil)
	if len(attrs) != 1 || attrs[0] != `data-attr="ok"` {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected click handler to remain after ApplyParams, got %d handlers", len(e.handlers))
	}
	if err := CallEventHandlers(e.Ui(), e, what.Click, "1 2 0 x"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
	if !called {
		t.Fatal("expected click handler to be called")
	}
}

func TestElement_ApplyGetter_EventHandlerAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testEventHandler{}
	if _, err := e.ApplyGetter(h); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if !e.HasTag(h) {
		t.Fatal("expected comparable event handler to be auto-tagged")
	}
	if err := CallEventHandlers(e.Ui(), e, what.Input, "name"); err != nil {
		t.Fatalf("expected event handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_ContextMenuHandlerAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testContextMenuHandler{}
	if _, err := e.ApplyGetter(h); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if !e.HasTag(h) {
		t.Fatal("expected comparable context menu handler to be auto-tagged")
	}
	if err := CallEventHandlers(e.Ui(), e, what.ContextMenu, "1 2 0 name"); err != nil {
		t.Fatalf("expected context menu handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_EventHandlerNonComparableNoAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testNonComparableEventHandler{names: []string{"name"}}
	if _, err := e.ApplyGetter(h); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable event handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.Ui(), e, what.Input, "name"); err != nil {
		t.Fatalf("expected event handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_ContextMenuHandlerNonComparableNoAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testNonComparableContextMenuHandler{names: []string{"name"}}
	if _, err := e.ApplyGetter(h); err != nil {
		t.Fatalf("ApplyGetter returned error: %v", err)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable context menu handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.Ui(), e, what.ContextMenu, "1 2 0 name"); err != nil {
		t.Fatalf("expected context menu handler to run, got %v", err)
	}
}

func TestElement_JawsInit(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{s: "foo"}
	tss.initError = tag.ErrNotComparable
	e := rq.NewElement(tss)

	gotTag, err := e.ApplyGetter(tss)
	is.Equal(atomic.LoadInt32(&tss.initCalled), int32(1))
	if gotTag != tss {
		t.Errorf("tag was %#v", gotTag)
	}
	if err != tag.ErrNotComparable {
		t.Error(err)
	}
}
