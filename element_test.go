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
	"github.com/matryer/is"
)

type testUi struct {
	renderCalled int32
	updateCalled int32
	getCalled    int32
	setCalled    int32
	s            string
	renderFn     func(e *Element, w io.Writer, params []any)
	updateFn     func(e *Element)
}

func (tss *testUi) JawsGetString(e *Element) string {
	atomic.AddInt32(&tss.getCalled, 1)
	return tss.s
}

func (tss *testUi) JawsSetString(e *Element, s string) error {
	atomic.AddInt32(&tss.setCalled, 1)
	tss.s = s
	return nil
}

func (tss *testUi) JawsRender(e *Element, w io.Writer, params []any) {
	e.Tag(tss)
	atomic.AddInt32(&tss.renderCalled, 1)
	if tss.renderFn != nil {
		tss.renderFn(e, w, params)
	}
}

func (tss *testUi) JawsUpdate(e *Element) {
	atomic.AddInt32(&tss.updateCalled, 1)
	if tss.updateFn != nil {
		tss.updateFn(e)
	}
}

func TestElement_Tag(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	tss := &testUi{}
	e := rq.NewElement(tss)
	is.True(!e.HasTag(Tag("zomg")))
	e.Tag(Tag("zomg"))
	is.True(e.HasTag(Tag("zomg")))
	is.True(strings.Contains(e.String(), "zomg"))
}

func TestElement_Queued(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
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
			replaceHtml := template.HTML(fmt.Sprintf("<div id=\"%s\"></div>", e.Jid().String()))
			e.Replace(replaceHtml)
			e.Delete()
			is.Equal(e.wsQueue, []wsMsg{
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
					Data: string(replaceHtml),
					Jid:  e.jid,
					What: what.Replace,
				},
				{
					Jid:  e.jid,
					What: what.Delete,
				},
			})
		},
	}

	pendingRq := rq.Jaws.NewRequest(httptest.NewRequest(http.MethodGet, "/", nil))
	pendingRq.UI(tss)

	rq.UI(tss)
	rq.Jaws.Dirty(tss)
	rq.Dirty(tss)
	tmr := time.NewTimer(testTimeout)
	for atomic.LoadInt32(&tss.updateCalled) < 1 {
		select {
		case <-tmr.C:
			is.Fail()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	is.Equal(tss.updateCalled, int32(1))
	is.Equal(tss.renderCalled, int32(2))
}

func TestElement_ReplacePanicsOnMissingId(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
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
