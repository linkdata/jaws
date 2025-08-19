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

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
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
var _ Setter[string] = (*testUi)(nil)
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

func TestElement_helpers(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{}
	e := rq.NewElement(tss)
	is.Equal(e.Jaws, rq.jw.Jaws)
	is.Equal(e.Request, rq.Request)
	is.Equal(e.Session(), nil)
	e.Set("foo", "bar") // no session, so no effect
	is.Equal(e.Get("foo"), nil)
}

func TestElement_Tag(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{}
	e := rq.NewElement(tss)
	is.True(!e.HasTag(Tag("zomg")))
	e.Tag(Tag("zomg"))
	is.True(e.HasTag(Tag("zomg")))
	s := e.String()
	if !strings.Contains(s, "zomg") {
		t.Error(s)
	}
}

func TestElement_Queued(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
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
			th.Equal(rq.wsQueue, []wsMsg{
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
	RequestWriter{pendingRq, httptest.NewRecorder()}.UI(tss)

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
	th.Equal(tss.updateCalled, int32(1))
	th.Equal(tss.renderCalled, int32(2))
}

func TestElement_ReplacePanicsOnMissingId(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
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

func TestElement_maybeDirty(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)

	changed, err := e.maybeDirty(e, nil)
	th.True(changed)
	th.NoErr(err)

	changed, err = e.maybeDirty(e, ErrValueUnchanged)
	th.Equal(changed, false)
	th.Equal(err, nil)

	changed, err = e.maybeDirty(e, ErrNotComparable)
	th.Equal(changed, false)
	th.Equal(err, ErrNotComparable)
}

type testClickHandler struct {
}

func (tch testClickHandler) JawsClick(e *Element, name string) (err error) {
	return nil
}

var _ ClickHandler = testClickHandler{}

func TestElement_ApplyGetter(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{s: "foo"}
	e := rq.NewElement(tss)

	var tch testClickHandler
	tag, err := e.ApplyGetter(tch)
	if tag != tch {
		t.Errorf("tag was %#v", tag)
	}
	if err != nil {
		t.Error(err)
	}
	is.Equal(len(e.handlers), 1)
}

func TestElement_JawsInit(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{s: "foo"}
	tss.initError = ErrNotComparable
	e := rq.NewElement(tss)

	tag, err := e.ApplyGetter(tss)
	is.Equal(atomic.LoadInt32(&tss.initCalled), int32(1))
	if tag != tss {
		t.Errorf("tag was %#v", tag)
	}
	if err != ErrNotComparable {
		t.Error(err)
	}
}
