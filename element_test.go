package jaws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
	s            string
	renderFn     func(elem *Element, w io.Writer, params []any) error
	updateFn     func(elem *Element)
}

var _ UI = (*testUi)(nil)

func (tss *testUi) JawsGet(elem *Element) string {
	atomic.AddInt32(&tss.getCalled, 1)
	return tss.s
}

func (tss *testUi) JawsSet(elem *Element, value string) error {
	atomic.AddInt32(&tss.setCalled, 1)
	tss.s = value
	return nil
}

func (tss *testUi) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
	elem.Tag(tss)
	atomic.AddInt32(&tss.renderCalled, 1)
	if tss.renderFn != nil {
		err = tss.renderFn(elem, w, params)
	}
	return
}

func (tss *testUi) JawsUpdate(elem *Element) {
	atomic.AddInt32(&tss.updateCalled, 1)
	if tss.updateFn != nil {
		tss.updateFn(elem)
	}
}

type testApplyGetterAll struct{}

func (a testApplyGetterAll) JawsGetTag() any { return tag.Tag("tg") }
func (a testApplyGetterAll) JawsClick(elem *Element, click Click) error {
	return ErrEventUnhandled
}

func (a testApplyGetterAll) JawsInput(elem *Element, value string) error {
	return ErrEventUnhandled
}

func (a testApplyGetterAll) JawsSet(elem *Element, value string) error {
	return ErrEventUnhandled
}

type testNilTagGetter struct{}

func (testNilTagGetter) JawsGetTag() any { return nil }

type testReentrantDebugTag struct {
	rq *Request
}

func (tag testReentrantDebugTag) String() string {
	tag.rq.SetConnectFn(nil)
	return "reentrant"
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

func TestElement_JsCallQueuesElementScopedCall(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		t.Fatal("NewRequest returned nil")
	}

	elem := rq.NewElement(&testUi{})
	elem.JsCall("fn\tpart", "{\n\"a\":1\n}")
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	if len(rq.wsQueue) != 1 {
		t.Fatalf("queued messages = %d, want 1", len(rq.wsQueue))
	}
	msg := rq.wsQueue[0]
	if msg.Jid != elem.Jid() {
		t.Fatalf("Jid = %v, want %v", msg.Jid, elem.Jid())
	}
	if msg.What != what.Call {
		t.Fatalf("What = %v, want %v", msg.What, what.Call)
	}
	if msg.Data != `fnpart={"a":1}` {
		t.Fatalf("Data = %q, want %q", msg.Data, `fnpart={"a":1}`)
	}
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
	// Element.String renders tags with tag.TagsString, whose per-tag form depends
	// on the build (the value in debug/-race, only the type otherwise).
	if !strings.Contains(s, tag.TagString(tag.Tag("zomg"))) {
		t.Error(s)
	}
}

func TestElement_Queued(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		th := newTestHelper(t)
		rq := newTestRequest(t)
		defer closeRequestInBubble(rq)

		tss := &testUi{
			updateFn: func(elem *Element) {
				child := rq.NewElement(&testUi{})
				child.Freeze()
				elem.SetAttr("hidden", "")
				elem.RemoveAttr("hidden")
				elem.SetClass("bah")
				elem.RemoveClass("bah")
				elem.SetValue("foo")
				elem.SetInner("meh")
				elem.Append("<div></div>")
				elem.InsertBefore(child, "<span>before</span>")
				elem.Remove(child)
				elem.Order([]jid.Jid{1, 2})
				replaceHTML := template.HTML(fmt.Sprintf("<div id=\"%s\"></div>", elem.Jid().String()))
				elem.Replace(replaceHTML)
				th.Equal(rq.wsQueue, []wire.WsMsg{
					{
						Data: "hidden\n",
						Jid:  elem.jid,
						What: what.SAttr,
					},
					{
						Data: "hidden",
						Jid:  elem.jid,
						What: what.RAttr,
					},
					{
						Data: "bah",
						Jid:  elem.jid,
						What: what.SClass,
					},
					{
						Data: "bah",
						Jid:  elem.jid,
						What: what.RClass,
					},
					{
						Data: "foo",
						Jid:  elem.jid,
						What: what.Value,
					},
					{
						Data: "meh",
						Jid:  elem.jid,
						What: what.Inner,
					},
					{
						Data: "<div></div>",
						Jid:  elem.jid,
						What: what.Append,
					},
					{
						Data: child.Jid().String() + "\n<span>before</span>",
						Jid:  elem.jid,
						What: what.Insert,
					},
					{
						Data: child.Jid().String(),
						Jid:  elem.jid,
						What: what.Remove,
					},
					{
						Data: fmt.Sprintf("%s %s", Jid(1).String(), Jid(2).String()),
						Jid:  elem.jid,
						What: what.Order,
					},
					{
						Data: string(replaceHTML),
						Jid:  elem.jid,
						What: what.Replace,
					},
				})
			},
		}

		pendingRq := rq.Jaws.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
		th.NoErr(testRequestWriter{rq: pendingRq, Writer: httptest.NewRecorder()}.UI(tss))

		th.NoErr(rq.UI(tss))
		rq.Jaws.Dirty(tss)
		rq.Dirty(tss)
		// The Serve loop only broadcasts what.Update when its updateTicker fires
		// (1ms in tests). Advance the fake clock past it, then let the process
		// loop drain the update and invoke JawsUpdate.
		time.Sleep(2 * time.Millisecond)
		synctest.Wait()
		n := atomic.LoadInt32(&tss.updateCalled)
		// Lower bound: synctest scheduling can collapse the two Dirty calls into a
		// single updateTicker fire or split them across two, so the count is 1 or 2.
		th.True(n >= 1)
		th.Equal(tss.renderCalled, int32(2))
		// Upper bound: with jw.dirty now empty the system quiesces, so no further
		// JawsUpdate occurs (this catches a runaway re-update regression).
		time.Sleep(2 * time.Millisecond)
		synctest.Wait()
		th.Equal(atomic.LoadInt32(&tss.updateCalled), n)
	})
}

func TestElement_ChildOperations(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.newRequest(nil)
	defer jw.recycle(rq)

	parent := rq.NewElement(&testUi{})
	child := rq.NewElement(&testUi{})
	childTag := tag.Tag("child")
	child.Tag(childTag)
	child.Freeze()

	parent.InsertBefore(child, "<span>new</span>")
	if child.Deleted() {
		t.Fatal("InsertBefore deleted its reference child")
	}
	parent.Remove(child)
	if !child.Deleted() {
		t.Fatal("Remove did not mark child deleted")
	}
	if got := rq.GetElementByJid(child.Jid()); got != nil {
		t.Fatalf("removed child remains registered: %v", got)
	}
	if got := rq.GetElements(childTag); len(got) != 0 {
		t.Fatalf("removed child remains in tag registry: %v", got)
	}

	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	want := []wire.WsMsg{
		{Jid: parent.Jid(), What: what.Insert, Data: child.Jid().String() + "\n<span>new</span>"},
		{Jid: parent.Jid(), What: what.Remove, Data: child.Jid().String()},
	}
	if !reflect.DeepEqual(rq.wsQueue, want) {
		t.Fatalf("child operation queue = %+v, want %+v", rq.wsQueue, want)
	}
}

func TestElement_ChildOperationsRejectInvalidElement(t *testing.T) {
	tests := []struct {
		name  string
		child func(parent *Element, other *Request) *Element
	}{
		{name: "nil", child: func(*Element, *Request) *Element { return nil }},
		{name: "self", child: func(parent *Element, _ *Request) *Element { return parent }},
		{name: "other request", child: func(_ *Element, other *Request) *Element {
			return other.NewElement(&testUi{})
		}},
		{name: "deleted", child: func(parent *Element, _ *Request) *Element {
			child := parent.Request.NewElement(&testUi{})
			parent.Request.DeleteElement(child)
			return child
		}},
		{name: "unregistered", child: func(parent *Element, _ *Request) *Element {
			return &Element{Request: parent.Request, jid: parent.Jid() + 100}
		}},
	}
	operations := []struct {
		name string
		call func(*Element, *Element)
	}{
		{name: "InsertBefore", call: func(parent, child *Element) {
			parent.InsertBefore(child, "<span>new</span>")
		}},
		{name: "Remove", call: func(parent, child *Element) { parent.Remove(child) }},
	}

	for _, tt := range tests {
		for _, operation := range operations {
			t.Run(tt.name+"/"+operation.name, func(t *testing.T) {
				jw, err := New()
				if err != nil {
					t.Fatal(err)
				}
				defer jw.Close()
				logger := &captureErrorLogger{}
				jw.Logger = logger
				rq := jw.newRequest(nil)
				defer jw.recycle(rq)
				other := jw.newRequest(nil)
				defer jw.recycle(other)
				parent := rq.NewElement(&testUi{})
				child := tt.child(parent, other)

				panicked := false
				func() {
					defer func() { panicked = recover() != nil }()
					operation.call(parent, child)
				}()
				if panicked != deadlock.Debug {
					t.Fatalf("panicked = %t, want %t", panicked, deadlock.Debug)
				}
				loggedErr := logger.next(t)
				if !errors.Is(loggedErr, ErrInvalidChildElement) {
					t.Fatalf("logged error = %v, want ErrInvalidChildElement", loggedErr)
				}
				rq.muQueue.Lock()
				queued := len(rq.wsQueue)
				rq.muQueue.Unlock()
				if queued != 0 {
					t.Fatalf("invalid child operation queued %d messages", queued)
				}
			})
		}
	}
}

func TestElement_ChildOperationsOnDeletedParentAreInert(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	rq := jw.newRequest(nil)
	defer jw.recycle(rq)
	parent := rq.NewElement(&testUi{})
	child := rq.NewElement(&testUi{})
	rq.DeleteElement(parent)

	parent.InsertBefore(child, "<span>new</span>")
	parent.Remove(child)
	awaitTestLoggerQueue(t, jw)

	if child.Deleted() {
		t.Fatal("deleted parent changed its live child")
	}
	if logged := logger.snapshot(); len(logged) != 0 {
		t.Fatalf("deleted parent reported errors: %v", logged)
	}
	rq.muQueue.Lock()
	queued := len(rq.wsQueue)
	rq.muQueue.Unlock()
	if queued != 0 {
		t.Fatalf("deleted parent queued %d child operations", queued)
	}
}

func TestElement_ReplaceRejectsMissingId(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	logger := &captureErrorLogger{}
	jw.Logger = logger
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	e := rq.NewElement(&testUi{s: "foo"})

	if deadlock.Debug {
		// Debug builds fail fast with a panic after queueing the misuse.
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic in debug build")
			}
		}()
	}
	e.Replace(template.HTML(`<div id="wrong"></div>`))
	if deadlock.Debug {
		t.Fatal("Replace should have panicked in debug build")
	}

	// Production builds (with a Logger) report it via MustLog and skip the replace.
	loggedErr := logger.next(t)
	if !strings.Contains(loggedErr.Error(), "expected HTML") {
		t.Fatalf("expected misuse logged containing %q, got %v", "expected HTML", loggedErr)
	}
	// The malformed replacement must not have been enqueued.
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	for _, msg := range rq.wsQueue {
		if msg.What == what.Replace {
			t.Fatalf("rejected Replace was enqueued: %+v", msg)
		}
	}
}

func TestElement_AttrHelpersRejectReservedId(t *testing.T) {
	tests := []struct {
		name string
		call func(e *Element)
	}{
		{"SetAttr id", func(e *Element) { e.SetAttr("id", "changed") }},
		{"SetAttr ID", func(e *Element) { e.SetAttr("ID", "changed") }},
		{"SetAttr Id", func(e *Element) { e.SetAttr("Id", "changed") }},
		{"RemoveAttr id", func(e *Element) { e.RemoveAttr("id") }},
		{"RemoveAttr iD", func(e *Element) { e.RemoveAttr("iD") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jw, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer jw.Close()
			logger := &captureErrorLogger{}
			jw.Logger = logger
			// A plain Request has no running process loop, so nothing drains
			// wsQueue underneath the assertion (unlike newTestRequest).
			rq := jw.newRequest(nil)
			defer jw.recycle(rq)
			e := rq.NewElement(&testUi{})

			// Wrap only the call so that in a debug build the fail-fast panic is
			// recovered here and the identity/queue assertions below still run.
			call := func() { tt.call(e) }
			if deadlock.Debug {
				func() {
					defer func() {
						if recover() == nil {
							t.Fatal("reserved attribute did not panic in a debug build")
						}
					}()
					call()
				}()
			} else {
				call()
			}

			// Both builds queue the misuse for logging and send nothing.
			loggedErr := logger.next(t)
			if !errors.Is(loggedErr, ErrReservedAttribute) {
				t.Fatalf("error = %v, want ErrReservedAttribute", loggedErr)
			}
			rq.muQueue.Lock()
			defer rq.muQueue.Unlock()
			for _, msg := range rq.wsQueue {
				if msg.What == what.SAttr || msg.What == what.RAttr {
					t.Fatalf("reserved attribute was enqueued: %+v", msg)
				}
			}
		})
	}
}

func TestElement_AttrHelpersAllowNormalAttr(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	// A plain Request has no running process loop, so nothing drains wsQueue
	// underneath the assertion (unlike newTestRequest).
	rq := jw.newRequest(nil)
	defer jw.recycle(rq)
	e := rq.NewElement(&testUi{})
	e.SetAttr("hidden", "yes")
	e.RemoveAttr("hidden")
	rq.muQueue.Lock()
	defer rq.muQueue.Unlock()
	var sattr, rattr int
	for _, msg := range rq.wsQueue {
		switch msg.What {
		case what.SAttr:
			sattr++
		case what.RAttr:
			rattr++
		}
	}
	if sattr != 1 || rattr != 1 {
		t.Fatalf("want 1 SAttr and 1 RAttr queued, got %d and %d", sattr, rattr)
	}
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

func TestElement_AddHandlersAfterRenderPanics(t *testing.T) {
	if !deadlock.Debug {
		t.Skip("AddHandlers assertion is only active in debug builds")
	}
	rq := newTestRequest(t)
	defer rq.Close()
	e := rq.NewElement(&testUi{})
	if err := e.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic adding handlers after JawsRender returned")
		}
	}()
	e.AddHandlers(struct{}{})
}

// assertHandlerMutationFrozen verifies that fn (a handler-adding call on a frozen
// Element) is rejected: debug builds panic, production builds report it via MustLog
// and drop the handler so the lock-free read on the event goroutine is never raced.
func assertHandlerMutationFrozen(t *testing.T, e *Element, fn func()) {
	t.Helper()
	before := len(e.handlers)
	if deadlock.Debug {
		defer func() {
			if recover() == nil {
				t.Error("expected panic when mutating handlers after freeze")
			}
		}()
		fn()
		return
	}
	fn()
	if got := len(e.handlers); got != before {
		t.Errorf("late handler not dropped: len(handlers) = %d, want %d", got, before)
	}
}

func TestElement_HandlersFrozenAfterRender(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	e := rq.NewElement(&testUi{})
	if err := e.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	if !e.frozen.Load() {
		t.Fatal("Element must be frozen after JawsRender returns")
	}
	assertHandlerMutationFrozen(t, e, func() { e.AddHandlers(testClickHandler{}) })
	assertHandlerMutationFrozen(t, e, func() { e.ApplyParams([]any{testEventHandler{}}) })
	assertHandlerMutationFrozen(t, e, func() { e.ApplyGetter(testClickHandler{}) })
}

func TestElement_FreezeSealsHandlers(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	e := rq.NewElement(&testUi{})
	e.Freeze()
	if !e.frozen.Load() {
		t.Fatal("Freeze must set the frozen flag")
	}
	assertHandlerMutationFrozen(t, e, func() { e.AddHandlers(testClickHandler{}) })
}

func TestElement_ApplyParamsAfterFreezeKeepsAttrs(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	e := rq.NewElement(&testUi{})
	if err := e.JawsRender(io.Discard, nil); err != nil {
		t.Fatal(err)
	}
	// Parsing attributes adds no handler, so it must not trip the freeze guard
	// in any build; the raw HTML attribute is still returned.
	attrs := e.ApplyParams([]any{template.HTMLAttr("hidden")})
	if len(attrs) != 1 || attrs[0] != template.HTMLAttr("hidden") {
		t.Fatalf("ApplyParams dropped attributes after freeze: %v", attrs)
	}
	if len(e.handlers) != 0 {
		t.Fatalf("expected no handlers, got %d", len(e.handlers))
	}
}

func TestElement_UnrenderedAcceptsHandlers(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	e := rq.NewElement(&testUi{})
	// An Element that has not been rendered (or frozen) still accepts handlers
	// from all three entry points; several event-dispatch tests rely on this.
	click := &testClickCounter{wantName: "name"}
	e.AddHandlers(testEventHandler{})
	e.ApplyParams([]any{testContextMenuHandler{}})
	e.ApplyGetter(click)
	if got := len(e.handlers); got != 3 {
		t.Fatalf("expected 3 handlers, got %d", got)
	}
	if err := CallEventHandlers(e.UI(), e, what.Click, "1 2 5 name"); err != nil {
		t.Fatalf("click dispatch failed: %v", err)
	}
	if click.n != 1 {
		t.Fatalf("expected click handler to fire once, got %d", click.n)
	}
}

func TestElement_RenderDebugAndDeletedBranches(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	tu := &testUi{renderFn: func(*Element, io.Writer, []any) error { return nil }}
	elem := rq.NewElement(tu)

	rq.mu.Lock()
	var sb strings.Builder
	err = elem.renderDebug(&sb)
	rq.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	elem.Tag(tag.Tag("a"), tag.Tag("b"))
	sb.Reset()
	if err = elem.renderDebug(&sb); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), ", ") {
		t.Fatal("expected comma-separated tags in debug output")
	}

	rq.Jaws.Debug = true
	sb.Reset()
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	rq.Jaws.Debug = false

	if elem.Deleted() {
		t.Fatal("element must not report Deleted before DeleteElement")
	}
	rq.DeleteElement(elem)
	if !elem.Deleted() {
		t.Fatal("element must report Deleted after DeleteElement")
	}
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatal(err)
	}
	elem.JawsUpdate()
}

func TestElement_JawsRenderDebugTagCanReenterRequest(t *testing.T) {
	// The re-entrancy this guards against only happens when renderDebug invokes the
	// tag's String method, which the default build never does; it applies in the
	// debug and -race builds where TagString renders values in full.
	if !tag.DebugRender {
		t.Skip("tag String is only invoked in debug/-race builds")
	}
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	jw.Debug = true
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))

	tu := &testUi{renderFn: func(elem *Element, _ io.Writer, _ []any) error {
		elem.Tag(testReentrantDebugTag{rq: rq})
		return nil
	}}
	elem := rq.NewElement(tu)
	var output strings.Builder
	rendered := make(chan error, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				rendered <- fmt.Errorf("render panic: %v", recovered)
			}
		}()
		rendered <- elem.JawsRender(&output, nil)
	}()

	select {
	case err = <-rendered:
		jw.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), "reentrant") {
			t.Fatalf("debug output = %q, want reentrant tag", output.String())
		}
	case <-time.After(time.Second):
		t.Fatal("debug tag String method deadlocked while re-entering Request")
	}
}

func TestElement_JawsRenderReturnsDebugWriteError(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	jw.Debug = true
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	elem := rq.NewElement(&testUi{})
	wantErr := errors.New("debug write failed")

	err = elem.JawsRender(&errResponseWriter{writeErr: wantErr}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("JawsRender error = %v, want %v", err, wantErr)
	}
}

func TestElement_RenderDebugSanitizesHTML5CommentClose(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	rq.Jaws.Debug = true

	tu := &testUi{renderFn: func(*Element, io.Writer, []any) error { return nil }}
	elem := rq.NewElement(tu)
	elem.Tag(tag.Tag("x--!>y"))

	var sb strings.Builder
	if err = elem.renderDebug(&sb); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sb.String(), "--!>") {
		t.Fatalf("HTML5 comment close escaped the debug comment: %q", sb.String())
	}
}

func TestElement_ApplyGetterDebugBranches(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()
	elem := rq.NewElement(&testUi{})

	if gotTag := elem.ApplyGetter(nil); gotTag != nil {
		t.Fatalf("unexpected tag %v", gotTag)
	}

	ag := testApplyGetterAll{}
	gotTags := elem.ApplyGetter(ag)
	if !elem.HasTag(tag.Tag("tg")) {
		t.Fatalf("missing Tag('tg') in %#v", gotTags)
	}
}

type testClickHandler struct{}

func (tch testClickHandler) JawsClick(elem *Element, click Click) (err error) {
	return nil
}

var _ ClickHandler = testClickHandler{}

type testNonComparableClickHandler struct {
	names []string
}

func (tch testNonComparableClickHandler) JawsClick(elem *Element, click Click) error {
	return nil
}

var _ ClickHandler = testNonComparableClickHandler{}

type testEventHandler struct{}

func (testEventHandler) JawsInput(elem *Element, value string) error {
	return nil
}

var _ InputHandler = testEventHandler{}

type testNonComparableEventHandler struct {
	names []string
}

func (testNonComparableEventHandler) JawsInput(elem *Element, value string) error {
	return nil
}

var _ InputHandler = testNonComparableEventHandler{}

type testContextMenuHandler struct{}

func (testContextMenuHandler) JawsContextMenu(elem *Element, click Click) error {
	return nil
}

var _ ContextMenuHandler = testContextMenuHandler{}

type testNonComparableContextMenuHandler struct {
	names []string
}

func (testNonComparableContextMenuHandler) JawsContextMenu(elem *Element, click Click) error {
	return nil
}

var _ ContextMenuHandler = testNonComparableContextMenuHandler{}

type testInitialHTMLAttrHandler struct {
	attr  template.HTMLAttr
	calls *int
}

func (h testInitialHTMLAttrHandler) JawsInitialHTMLAttr(elem *Element) (s template.HTMLAttr) {
	if h.calls != nil {
		(*h.calls)++
	}
	s = h.attr
	return
}

var _ InitialHTMLAttrHandler = testInitialHTMLAttrHandler{}

type testStringSetterWithInitialHTMLAttr struct {
	s    string
	attr template.HTMLAttr
}

func (s *testStringSetterWithInitialHTMLAttr) JawsGet(elem *Element) (value string) {
	value = s.s
	return
}

func (s *testStringSetterWithInitialHTMLAttr) JawsSet(elem *Element, value string) (err error) {
	s.s = value
	return
}

func (s *testStringSetterWithInitialHTMLAttr) JawsInitialHTMLAttr(elem *Element) (attr template.HTMLAttr) {
	attr = s.attr
	return
}

type testClickAndInitialHTMLAttr struct {
	clickCalled *bool
	attrCalls   *int
	attr        template.HTMLAttr
}

func (h testClickAndInitialHTMLAttr) JawsClick(elem *Element, click Click) error {
	if h.clickCalled != nil {
		*h.clickCalled = true
	}
	return nil
}

func (h testClickAndInitialHTMLAttr) JawsInitialHTMLAttr(elem *Element) (attr template.HTMLAttr) {
	if h.attrCalls != nil {
		(*h.attrCalls)++
	}
	attr = h.attr
	return
}

var (
	_ ClickHandler           = testClickAndInitialHTMLAttr{}
	_ InitialHTMLAttrHandler = testClickAndInitialHTMLAttr{}
)

type testUnhashableUI struct {
	m map[string]int
}

func (testUnhashableUI) JawsRender(elem *Element, w io.Writer, params []any) error { return nil }
func (testUnhashableUI) JawsUpdate(elem *Element)                                  {}

func TestElement_ApplyGetter(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest(t)
	defer rq.Close()

	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)

	var tch testClickHandler
	gotTag := e.ApplyGetter(tch)
	if gotTag != tch {
		t.Errorf("tag was %#v", gotTag)
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
	e.ApplyGetter(tch)
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.UI(), e, what.Click, "1 2 0 name"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_NonComparableHandler_NilLogger(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	if jw.Logger != nil {
		t.Fatal("expected nil Logger by default")
	}

	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	e := rq.NewElement(&testUi{s: "x"})
	tch := testNonComparableClickHandler{names: []string{"name"}}
	gotTag := e.ApplyGetter(tch)
	if gotTag != nil {
		t.Fatalf("expected declined non-comparable candidate to return a nil tag, got %#v", gotTag)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable handler to not be auto-tagged, got %v", got)
	}
	// The returned tag is what a widget stores and later dirties. A nil tag must
	// not reach MustLog (which panics with a nil Logger); a non-comparable one would.
	e.Dirty(gotTag)
}

func TestElement_ApplyGetter_NonComparableHandler_NoLog(t *testing.T) {
	jw, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer jw.Close()
	var buf bytes.Buffer
	jw.Logger = slog.New(slog.NewTextHandler(&buf, nil))

	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	e := rq.NewElement(&testUi{s: "x"})
	tch := testNonComparableClickHandler{names: []string{"name"}}
	e.ApplyGetter(tch)
	awaitTestLoggerQueue(t, jw)
	if strings.Contains(buf.String(), "not usable as tag") {
		t.Fatalf("expected no not-usable-as-tag log, got %q", buf.String())
	}
}

func TestElement_ApplyGetter_NilTagGetter(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(&testUi{s: "foo"})
	gotTag := e.ApplyGetter(testNilTagGetter{})
	if gotTag != nil {
		t.Fatalf("expected nil tag, got %#v", gotTag)
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected nil tag getter to not tag element, got %v", got)
	}

	e.Tag(nil)
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected explicit nil tag to be ignored, got %v", got)
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
	if err := CallEventHandlers(e.UI(), e, what.Click, "1 2 0 name"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
}

func TestElement_ApplyParams_SliceTags(t *testing.T) {
	tests := []struct {
		name  string
		param any
	}{
		{name: "any slice", param: []any{tag.Tag("a"), tag.Tag("b")}},
		{name: "tag slice", param: []tag.Tag{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rq := newTestRequest(t)
			defer rq.Close()

			e := rq.NewElement(testDivWidget{inner: "x"})
			e.ApplyParams([]any{tt.param})

			for _, tagValue := range []tag.Tag{"a", "b"} {
				if !e.HasTag(tagValue) {
					t.Fatalf("expected element to be tagged with %q; got %v", tagValue, rq.TagsOf(e))
				}
			}
		})
	}
}

func TestElement_ApplyParams_InitialHTMLAttrHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	attrCalls := 0
	attrs := e.ApplyParams([]any{
		"hidden",
		testInitialHTMLAttrHandler{attr: `data-attr="ok"`, calls: &attrCalls},
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
	if attrCalls != 0 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 0", attrCalls)
	}
}

func TestElement_ApplyParams_IgnoreInitialHTMLAttrOnCombinedParamHandler(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	clickCalled := false
	attrCalls := 0
	h := testClickAndInitialHTMLAttr{
		clickCalled: &clickCalled,
		attrCalls:   &attrCalls,
		attr:        `data-attr="ignored"`,
	}

	attrs := e.ApplyParams([]any{h})
	if len(attrs) != 0 {
		t.Fatalf("expected no attrs from parameter handlers, got %#v", attrs)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected click handler to be registered, got %d handlers", len(e.handlers))
	}
	if err := CallEventHandlers(e.UI(), e, what.Click, "1 2 0 x"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
	if !clickCalled {
		t.Fatal("expected click handler to be called")
	}
	if attrCalls != 0 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 0", attrCalls)
	}
}

func TestElement_ApplyInitialHTMLAttr(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	if attrs := e.ApplyInitialHTMLAttr(nil); len(attrs) != 0 {
		t.Fatalf("unexpected attrs for nil getter: %#v", attrs)
	}

	attrCalls := 0
	h := testInitialHTMLAttrHandler{calls: &attrCalls}
	if attrs := e.ApplyInitialHTMLAttr(h); len(attrs) != 0 {
		t.Fatalf("empty attr must yield no attrs, got %#v", attrs)
	}
	if attrCalls != 1 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 1", attrCalls)
	}

	h.attr = `data-attr="ok"`
	attrs := e.ApplyInitialHTMLAttr(h)
	if len(attrs) != 1 || attrs[0] != h.attr {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if attrCalls != 2 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 2", attrCalls)
	}
}

func TestElement_ApplyInitialHTMLAttr_RendersGetterAttribute(t *testing.T) {
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
		t.Fatalf("expected no handlers, got %d handlers", len(rq.elems[0].handlers))
	}
}

func TestElement_ApplyGetter_DoesNotApplyInitialHTMLAttr(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	clickCalled := false
	attrCalls := 0
	h := testClickAndInitialHTMLAttr{
		clickCalled: &clickCalled,
		attrCalls:   &attrCalls,
		attr:        `data-attr="ok"`,
	}
	if gotTag := e.ApplyGetter(h); gotTag != h {
		t.Fatalf("tag = %#v, want %#v", gotTag, h)
	}
	if attrCalls != 0 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 0", attrCalls)
	}
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if !e.HasTag(h) {
		t.Fatal("expected combined handler to be tagged")
	}
	if err := CallEventHandlers(e.UI(), e, what.Click, "1 2 0 x"); err != nil {
		t.Fatalf("expected click handler to run, got %v", err)
	}
	if !clickCalled {
		t.Fatal("expected click handler to be called")
	}
}

func TestElement_ApplyInitialHTMLAttr_CombinedHandlerOnlyAppliesAttr(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	clickCalled := false
	attrCalls := 0
	h := testClickAndInitialHTMLAttr{
		clickCalled: &clickCalled,
		attrCalls:   &attrCalls,
		attr:        `data-attr="ok"`,
	}
	attrs := e.ApplyInitialHTMLAttr(h)
	if len(attrs) != 1 || attrs[0] != h.attr {
		t.Fatalf("unexpected attrs: %#v", attrs)
	}
	if attrCalls != 1 {
		t.Fatalf("JawsInitialHTMLAttr called %d times, want 1", attrCalls)
	}
	if clickCalled {
		t.Fatal("click handler was called")
	}
	if len(e.handlers) != 0 {
		t.Fatalf("expected no handlers, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestElement_ApplyGetter_InputHandlerAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testEventHandler{}
	e.ApplyGetter(h)
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if !e.HasTag(h) {
		t.Fatal("expected comparable input handler to be auto-tagged")
	}
	if err := CallEventHandlers(e.UI(), e, what.Input, "name"); err != nil {
		t.Fatalf("expected input handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_ContextMenuHandlerAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testContextMenuHandler{}
	e.ApplyGetter(h)
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if !e.HasTag(h) {
		t.Fatal("expected comparable context menu handler to be auto-tagged")
	}
	if err := CallEventHandlers(e.UI(), e, what.ContextMenu, "1 2 0 name"); err != nil {
		t.Fatalf("expected context menu handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_InputHandlerNonComparableNoAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testNonComparableEventHandler{names: []string{"name"}}
	e.ApplyGetter(h)
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable input handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.UI(), e, what.Input, "name"); err != nil {
		t.Fatalf("expected input handler to run, got %v", err)
	}
}

func TestElement_ApplyGetter_ContextMenuHandlerNonComparableNoAutoTag(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	e := rq.NewElement(testDivWidget{inner: "x"})
	h := testNonComparableContextMenuHandler{names: []string{"name"}}
	e.ApplyGetter(h)
	if len(e.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(e.handlers))
	}
	if got := rq.TagsOf(e); len(got) != 0 {
		t.Fatalf("expected non-comparable context menu handler to not be auto-tagged, got %v", got)
	}
	if err := CallEventHandlers(e.UI(), e, what.ContextMenu, "1 2 0 name"); err != nil {
		t.Fatalf("expected context menu handler to run, got %v", err)
	}
}

// nonReflexiveUI is a comparable UI value that is not equal to itself when it holds
// NaN; it backs TestNewErrUnusableUI.
type nonReflexiveUI struct{ f float64 }

func (nonReflexiveUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (nonReflexiveUI) JawsUpdate(*Element)                         {}

type testElementState struct{ name string }

func TestElementState_ClaimOnce(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})
	if got := ElementState(elem); got != nil {
		t.Fatalf("unclaimed slot = %v, want nil", got)
	}

	first := &testElementState{name: "first"}
	if err := SetElementState(elem, first); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if got := ElementState(elem); got != first {
		t.Fatalf("loaded %v, want the claimed state", got)
	}

	// A second claim fails and leaves the original in place, including one carrying the
	// same dynamic type: same type does not mean same owner.
	for _, state := range []any{&testElementState{name: "same type"}, "other type"} {
		if err := SetElementState(elem, state); !errors.Is(err, ErrElementStateClaimed) {
			t.Fatalf("second claim with %T = %v, want %v", state, err, ErrElementStateClaimed)
		}
	}
	if got := ElementState(elem); got != first {
		t.Fatalf("state after rejected claims = %v, want the original", got)
	}

	// Slots are per Element.
	other := rq.NewElement(&testUi{})
	if got := ElementState(other); got != nil {
		t.Fatalf("second element's slot = %v, want nil", got)
	}
	if err := SetElementState(other, &testElementState{name: "second"}); err != nil {
		t.Fatalf("claiming a different element: %v", err)
	}
	if ElementState(elem) != first {
		t.Error("claiming another element disturbed the first element's state")
	}
}

func TestElementState_NilHandling(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})

	// A nil interface cannot be stored: it is indistinguishable from an unclaimed slot,
	// so accepting it would report success while leaving the slot claimable.
	if err := SetElementState(elem, nil); !errors.Is(err, ErrElementStateNil) {
		t.Fatalf("nil claim = %v, want %v", err, ErrElementStateNil)
	}
	if got := ElementState(elem); got != nil {
		t.Fatalf("slot after nil claim = %v, want still nil", got)
	}

	// A typed nil is a non-nil interface, so it does claim the slot.
	var typedNil *testElementState
	if err := SetElementState(elem, typedNil); err != nil {
		t.Fatalf("typed-nil claim: %v", err)
	}
	if err := SetElementState(elem, &testElementState{}); !errors.Is(err, ErrElementStateClaimed) {
		t.Fatalf("claim after typed nil = %v, want %v", err, ErrElementStateClaimed)
	}

	// The nil-argument check precedes the occupancy check, so a nil state against an
	// occupied slot still reports ErrElementStateNil.
	if err := SetElementState(elem, nil); !errors.Is(err, ErrElementStateNil) {
		t.Fatalf("nil claim on occupied slot = %v, want %v", err, ErrElementStateNil)
	}
}

func TestElementState_ConcurrentClaims(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(&testUi{})
	const claimants = 8

	var wg sync.WaitGroup
	errs := make([]error, claimants)
	states := make([]any, claimants)
	start := make(chan struct{})
	for i := range claimants {
		states[i] = &testElementState{name: "claimant"}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = SetElementState(elem, states[i])
		}()
	}
	close(start)
	wg.Wait()

	var winners int
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
			if got := ElementState(elem); got != states[i] {
				t.Errorf("claimant %d succeeded but the slot holds %v", i, got)
			}
		case !errors.Is(err, ErrElementStateClaimed):
			t.Errorf("claimant %d = %v, want %v", i, err, ErrElementStateClaimed)
		}
	}
	if winners != 1 {
		t.Errorf("successful claims = %d, want exactly 1", winners)
	}
}

// benchCreateUI is a stateless widget that never touches the state slot, so the
// benchmark measures the per-Element cost rather than Template bookkeeping. It
// supports multiple live Elements because one value backs every benchmark Element.
type benchCreateUI struct{}

func (benchCreateUI) JawsRender(elem *Element, w io.Writer, params []any) error {
	_, err := io.WriteString(w, "<span>x</span>")
	return err
}

func (benchCreateUI) JawsUpdate(elem *Element) {}

// BenchmarkElementCreateBatch creates and renders a fixed batch of Elements per iteration,
// then deletes them with the timer stopped.
//
// Batching is deliberate: b.StopTimer and b.StartTimer each call runtime.ReadMemStats, so
// toggling around a single sub-microsecond creation would leave the timed section tiny,
// calibration would pick an enormous iteration count, and the excluded setup would run
// for minutes.
// Amortising both calls over the batch keeps that honest, and deleting the batch keeps the
// Request registry bounded instead of growing across iterations. The reported figure is per
// batch of 64 Elements.
//
// [testing.B.Loop] excludes construction and the deferred Close from the measurement,
// while the explicit timer calls exclude per-batch deletion.
func BenchmarkElementCreateBatch(b *testing.B) {
	b.ReportAllocs()
	const batch = 64

	jw, err := New()
	if err != nil {
		b.Fatal(err)
	}
	defer jw.Close()
	rq := jw.newRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	if rq == nil {
		b.Fatal("nil request")
	}
	var ui benchCreateUI
	elems := make([]*Element, 0, batch)

	for b.Loop() {
		elems = elems[:0]
		for range batch {
			elem := rq.NewElement(ui)
			if err := elem.JawsRender(io.Discard, nil); err != nil {
				b.Fatal(err)
			}
			elems = append(elems, elem)
		}
		b.StopTimer()
		rq.DeleteElements(elems)
		b.StartTimer()
	}
}

// ifaceSliceUI is statically comparable (an interface field) but panics when compared
// at runtime, since the interface holds a slice.
type ifaceSliceUI struct{ v any }

func (ifaceSliceUI) JawsRender(*Element, io.Writer, []any) error { return nil }
func (ifaceSliceUI) JawsUpdate(*Element)                         {}

// typedNilUI has pointer-receiver methods that tolerate a nil receiver, so a typed nil
// (*typedNilUI)(nil) is a usable UI value that renders without dereferencing.
type typedNilUI struct{ s string }

func (u *typedNilUI) JawsRender(_ *Element, w io.Writer, _ []any) error {
	s := "typednil"
	if u != nil {
		s = u.s
	}
	_, err := io.WriteString(w, s)
	return err
}
func (*typedNilUI) JawsUpdate(*Element) {}

func TestNewErrUnusableUI(t *testing.T) {
	tests := []struct {
		name    string
		ui      UI
		wantErr bool
	}{
		{"nil", nil, true},
		{"nan struct", nonReflexiveUI{f: math.NaN()}, true},
		{"map field (statically incomparable)", testUnhashableUI{m: map[string]int{"x": 1}}, true},
		{"interface holding slice (runtime-incomparable)", ifaceSliceUI{v: []int{1}}, true},
		{"valid pointer", &testUi{}, false},
		{"valid struct", nonReflexiveUI{f: 1.5}, false},
		// A typed nil is comparable and equal to itself, so it is usable; only a nil
		// interface is rejected.
		{"typed nil pointer", (*typedNilUI)(nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewErrUnusableUI(tt.ui)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("NewErrUnusableUI = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("NewErrUnusableUI = nil, want error")
			}
			// The error stands in for both tag identities so callers can match either.
			if !errors.Is(err, tag.ErrNotUsableAsTag) {
				t.Errorf("err does not match tag.ErrNotUsableAsTag: %v", err)
			}
			if !errors.Is(err, tag.ErrNotComparable) {
				t.Errorf("err does not match tag.ErrNotComparable: %v", err)
			}
		})
	}
}

// TestNewElementNilUIRendersNoop verifies that NewElement(nil) does not terminate the
// Request — a nil UI is never reconciled by a container, so it is harmless — and
// returns an Element that renders and updates as a no-op rather than panicking on the
// nil UI. A nil child returned from a container is instead rejected; see
// TestContainerTerminatesOnUnusableChild.
func TestNewElementNilUIRendersNoop(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	elem := rq.NewElement(nil)
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("NewElement(nil) cancelled the Request: %v", cause)
	}

	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("JawsRender err = %v, want nil", err)
	}
	if sb.Len() != 0 {
		t.Fatalf("nil-UI render wrote %q, want empty", sb.String())
	}
	elem.JawsUpdate() // must not panic
}

// TestNewElementTypedNilUIDispatchesToRenderer documents that a typed nil UI (a
// non-nil interface holding a nil pointer) is usable — comparable and equal to itself
// — and dispatches to its Renderer rather than being treated as unusable. Tolerating
// the nil receiver is the concrete type's responsibility; typedNilUI does so.
func TestNewElementTypedNilUIDispatchesToRenderer(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	var ui UI = (*typedNilUI)(nil)
	if err := NewErrUnusableUI(ui); err != nil {
		t.Fatalf("NewErrUnusableUI(typed nil) = %v, want nil (usable)", err)
	}

	elem := rq.NewElement(ui)
	if cause := context.Cause(rq.Context()); cause != nil {
		t.Fatalf("NewElement(typed nil) cancelled the Request: %v", cause)
	}
	var sb strings.Builder
	if err := elem.JawsRender(&sb, nil); err != nil {
		t.Fatalf("JawsRender err = %v", err)
	}
	if sb.String() != "typednil" {
		t.Fatalf("render = %q, want %q", sb.String(), "typednil")
	}
}
