package jaws

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

const testTimeout = time.Second * 3

func fillWsCh(ch chan string) {
	for {
		select {
		case ch <- "":
		default:
			return
		}
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	is.Equal(rq.wantMessage(&Message{Dest: Tag("sometag")}), false)
	jid := rq.Register("sometag")
	is.True(jid.IsValid())
	is.Equal(rq.wantMessage(&Message{Dest: Tag("sometag")}), true)

	jid2 := rq.Register(jid)
	is.Equal(jid, jid2)

	jid = rq.Register("foo")
	is.True(jid.IsValid())

	jid2 = rq.Register("")
	is.True(jid2.IsValid())
	is.True(jid != jid2)
}

func TestRequest_HeadHTML(t *testing.T) {
	is := newTestHelper(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	var sb strings.Builder
	rq.Writer(&sb).HeadHTML()
	txt := sb.String()
	is.Equal(strings.Contains(string(txt), rq.JawsKeyString()), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	jid := rq.Register("foo")
	rq.jw.Broadcast(Message{Dest: Tag("foo"), What: what.Inner, Data: "bar"})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case msgstr := <-rq.outCh:
		msg, ok := wsParse([]byte(msgstr))
		is.True(ok)
		elem := rq.getElementByJid(jid)
		is.True(elem != nil)
		is.Equal(msg, wsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
	}
}

func TestRequest_OutboundRespectsJawsClosed(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	jw := rq.jw
	var callCount int32
	tag := Tag("foo")
	rq.ctx = context.Background()
	rq.Register(tag, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		th.Equal(1, jw.RequestCount())
		jw.Close()
		return nil
	})
	fillWsCh(rq.outCh)
	jw.Broadcast(Message{Dest: Tag("foo"), What: what.Hook, Data: "bar"})
	select {
	case <-th.C:
		th.Equal(int(atomic.LoadInt32(&callCount)), 0)
		th.Timeout()
	case <-rq.Done():
		th.Error("request ctx cancelled too soon")
	case <-jw.Done():
	}
	th.Equal(int(atomic.LoadInt32(&callCount)), 1)
	select {
	case <-rq.Done():
		th.Error("request ctx cancelled too soon")
	default:
	}
	if jw.log.Len() != 0 {
		t.Errorf("%q", jw.log.String())
	}
}

func TestRequest_OutboundRespectsContextDone(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	var callCount int32
	tag := Tag("foo")
	rq.Register(tag, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel()
		return errors.New(val)
	})
	fillWsCh(rq.outCh)
	rq.jw.Broadcast(Message{Dest: Tag("foo"), What: what.Hook, Data: "bar"})

	select {
	case <-th.C:
		th.Equal(int(atomic.LoadInt32(&callCount)), 0)
		th.Timeout()
	case <-rq.jw.Done():
		th.Fatal("jaws done too soon")
	case <-rq.ctx.Done():
	}

	th.Equal(int(atomic.LoadInt32(&callCount)), 1)

	select {
	case <-rq.jw.Done():
		th.Fatal("jaws done too soon")
	default:
	}
}

func TestRequest_OutboundOverflowPanicsWithNoLogger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	rq.expectPanic = true
	rq.jw.Logger = nil
	defer rq.Close()
	rq.Register(Tag("foo"))
	fillWsCh(rq.outCh)
	rq.Jaws.Broadcast(Message{Dest: Tag("foo"), What: what.Inner, Data: "bar"})
	select {
	case <-th.C:
		th.Timeout()
	case <-rq.doneCh:
		th.Equal(len(rq.outCh), cap(rq.outCh))
		th.True(rq.panicked)
	}
}

func TestRequest_Trigger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	gotFooCall := make(chan struct{})
	gotEndCall := make(chan struct{})
	rq.Register("foo", func(e *Element, evt what.What, val string) error {
		defer close(gotFooCall)
		return nil
	})
	rq.Register(("err"), func(e *Element, evt what.What, val string) error {
		return errors.New(val)
	})
	rq.Register(("end"), func(e *Element, evt what.What, val string) error {
		defer close(gotEndCall)
		return nil
	})

	// broadcasts from ourselves should not invoke fn
	// rq.Broadcast(Message{Dest: Tag("foo"), What: what.Input, Data: "bar"})
	// rq.Broadcast(Message{Dest: Tag("err"), What: what.Input, Data: "baz"})
	rq.jw.Broadcast(Message{Dest: Tag("end"), What: what.Input, Data: ""}) // to know when to stop
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		th.Fatal(s)
	case <-gotFooCall:
		th.Fatal("gotFooCall")
	case <-gotEndCall:
	}

	// global broadcast should invoke fn
	rq.jw.Broadcast(Message{Dest: Tag("foo"), What: what.Input, Data: "bar"})
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		th.Fatal(s)
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.jw.Broadcast(Message{Dest: Tag("err"), What: what.Input, Data: "omg"})
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		th.Equal(msg, (&wsMsg{
			Data: "danger\nomg",
			Jid:  jid.Jid(0),
			What: what.Alert,
		}).Format())
	}
}

func TestRequest_EventFnQueue(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	// calls to slow event functions queue up and are executed in order
	firstDoneCh := make(chan struct{})
	var sleepDone int32
	var callCount int32
	rq.Register(("sleep"), func(e *Element, evt what.What, val string) error {
		count := int(atomic.AddInt32(&callCount, 1))
		if val != strconv.Itoa(count) {
			t.Logf("val=%s, count=%d, cap=%d", val, count, cap(rq.outCh))
			th.Fail()
		}
		if count == 1 {
			close(firstDoneCh)
		}
		for atomic.LoadInt32(&sleepDone) == 0 {
			time.Sleep(time.Millisecond)
		}
		return nil
	})

	for i := 0; i < cap(rq.outCh); i++ {
		rq.jw.Broadcast(Message{Dest: Tag("sleep"), What: what.Input, Data: strconv.Itoa(i + 1)})
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-rq.doneCh:
		th.Fatal("doneCh")
	case <-firstDoneCh:
	}

	th.Equal(atomic.LoadInt32(&callCount), int32(1))
	atomic.StoreInt32(&sleepDone, 1)
	th.Equal(rq.panicVal, nil)

	for int(atomic.LoadInt32(&callCount)) < cap(rq.outCh) {
		select {
		case <-th.C:
			t.Logf("callCount=%d, cap=%d", atomic.LoadInt32(&callCount), cap(rq.outCh))
			th.Equal(rq.panicVal, nil)
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	th.Equal(atomic.LoadInt32(&callCount), int32(cap(rq.outCh)))
}

func TestRequest_EventFnQueueOverflowPanicsWithNoLogger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	var wait int32

	rq.Register(("bomb"), func(e *Element, evt what.What, val string) error {
		time.Sleep(time.Millisecond * time.Duration(atomic.AddInt32(&wait, 1)))
		return nil
	})

	rq.expectPanic = true
	rq.jw.Logger = nil
	jid := jidForTag(rq.Request, Tag("bomb"))

	for {
		select {
		case <-rq.doneCh:
			th.True(rq.panicked)
			th.True(strings.Contains(rq.panicVal.(error).Error(), "eventCallCh is full sending"))
			return
		case <-th.C:
			th.Timeout()
		case rq.inCh <- wsMsg{Jid: jid, What: what.Input}:
		}
	}
}

func TestRequest_IgnoresIncomingMsgsDuringShutdown(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	var spewState int32
	var callCount int32
	rq.Register(("spew"), func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		if len(rq.outCh) < cap(rq.outCh) {
			rq.jw.Broadcast(Message{Dest: Tag("spew"), What: what.Input})
		} else {
			atomic.StoreInt32(&spewState, 1)
			for atomic.LoadInt32(&spewState) == 1 {
				time.Sleep(time.Millisecond)
			}
		}
		return errors.New("chunks")
	})
	rq.Register(Tag("foo"))

	rq.jw.Broadcast(Message{Dest: Tag("spew"), What: what.Input})

	// wait for the event fn to be in hold state
	waited := 0
	for waited < 1000 && atomic.LoadInt32(&spewState) == 0 {
		time.Sleep(time.Millisecond)
		waited++
	}
	th.Equal(atomic.LoadInt32(&spewState), int32(1))
	th.Equal(cap(rq.outCh), len(rq.outCh))
	th.True(waited < 1000)

	// sending a message will now fail the rq since the
	// outbound channel is full, but with the
	// event fn holding it won't be able to end
	select {
	case rq.bcastCh <- Message{Dest: Tag("foo"), What: what.Inner, Data: ""}:
	case <-th.C:
		th.Timeout()
	case <-rq.doneCh:
		th.Fatal()
	}

	// rq should now be in shutdown phase draining channels
	// while waiting for the event fn to return
	for i := 0; i < cap(rq.outCh)*2; i++ {
		select {
		case <-rq.doneCh:
			th.Fatal()
		case <-th.C:
			th.Timeout()
		default:
			rq.Jaws.Broadcast(Message{Dest: rq})
		}
		select {
		case rq.inCh <- wsMsg{}:
		case <-rq.doneCh:
			th.Fatal()
		case <-th.C:
			th.Timeout()
		}
	}

	// release the event fn
	atomic.StoreInt32(&spewState, 2)

	select {
	case <-rq.doneCh:
		th.True(atomic.LoadInt32(&callCount) > 1)
	case <-th.C:
		th.Timeout()
	}

	// log data should contain message that we were unable to deliver error
	th.True(strings.Contains(rq.jw.log.String(), "outboundMsgCh full sending event"))
}

func TestRequest_Alert(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	rq1.Alert("info", "<html>\nnot\tescaped")
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq1.outCh:
		if s != "Alert\t\t\"info\\n<html>\\nnot\\tescaped\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case s := <-rq2.outCh:
		t.Errorf("%q", s)
	default:
	}
}

func TestRequest_Redirect(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	rq1.Redirect("some-url")
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq1.outCh:
		if s != "Redirect\t\t\"some-url\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case s := <-rq2.outCh:
		t.Errorf("%q", s)
	default:
	}
}

func TestRequest_AlertError(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq := tj.newRequest(nil)
	rq.AlertError(errors.New("<html>\nshould-be-escaped"))
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		if s != "Alert\t\t\"danger\\n&lt;html&gt;\\nshould-be-escaped\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func TestRequest_DeleteByTag(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	nextJid = 0
	rq1 := tj.newRequest(nil)
	ui1 := &testUi{}
	e11 := rq1.NewElement(ui1)
	th.Equal(e11.jid, Jid(1))
	e11.Tag(Tag("e11"), Tag("foo"))
	e12 := rq1.NewElement(ui1)
	th.Equal(e12.jid, Jid(2))
	e12.Tag(Tag("e12"))
	e13 := rq1.NewElement(ui1)
	th.Equal(e13.jid, Jid(3))
	e13.Tag(Tag("e13"), Tag("bar"))

	rq2 := tj.newRequest(nil)
	ui2 := &testUi{}
	e21 := rq2.NewElement(ui2)
	th.Equal(e21.jid, Jid(4))
	e21.Tag(Tag("e21"), Tag("foo"))
	e22 := rq2.NewElement(ui2)
	th.Equal(e22.jid, Jid(5))
	e22.Tag(Tag("e22"))
	e23 := rq2.NewElement(ui2)
	th.Equal(e23.jid, Jid(6))
	e23.Tag(Tag("e23"))

	tj.Delete([]any{Tag("foo"), Tag("bar"), Tag("nothere"), Tag("e23")})

	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq1.outCh:
		if s != "Delete\tJid.1\t\"\"\nDelete\tJid.3\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq2.outCh:
		if s != "Delete\tJid.4\t\"\"\nDelete\tJid.6\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func TestRequest_HtmlIdBroadcast(t *testing.T) {
	th := newTestHelper(t)
	tj := newTestJaws()
	defer tj.Close()
	rq1 := tj.newRequest(nil)
	rq2 := tj.newRequest(nil)

	tj.Broadcast(Message{
		Dest: "fooId",
		What: what.Inner,
		Data: "inner",
	})
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq1.outCh:
		if s != "Inner\tfooId\t\"inner\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq2.outCh:
		if s != "Inner\tfooId\t\"inner\"\n" {
			t.Errorf("%q", s)
		}
	}
}

func jidForTag(rq *Request, tag any) jid.Jid {
	if elems := rq.GetElements(tag); len(elems) > 0 {
		return elems[0].jid
	}
	return 0
}

func TestRequest_ConnectFn(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	th.Equal(rq.GetConnectFn(), nil)
	th.NoErr(rq.onConnect())

	wantErr := errors.New("getouttahere")
	fn := func(rq *Request) error {
		return wantErr
	}
	rq.SetConnectFn(fn)
	th.Equal(rq.onConnect(), wantErr)
}

func TestRequest_WsQueueOverflowCancels(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	elem := rq.NewElement(NewUiDiv(makeHtmlGetter("foo")))
	go func() {
		for i := 0; i < maxWsQueueLengthPerElement*10; i++ {
			elem.SetInner(template.HTML(strconv.Itoa(i)))
		}
	}()
	select {
	case <-th.C:
		th.Timeout()
	case <-rq.Done():
	}
	th.True(errors.Is(context.Cause(rq.Context()), ErrWebsocketQueueOverflow))
}

func TestRequest_Dirty(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{s: "foo"}
	rq.UI(NewUiText(tss))
	th.Equal(tss.getCalled, int32(1))
	th.True(strings.Contains(string(rq.BodyString()), "foo"))

	rq.Dirty(tss)
	for atomic.LoadInt32(&tss.getCalled) < 2 {
		select {
		case <-th.C:
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestRequest_UpdatePanicLogs(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{
		updateFn: func(e *Element) {
			panic("wildpanic")
		}}
	rq.UI(tss)
	rq.Dirty(tss)
	select {
	case <-th.C:
		th.Timeout()
	case <-rq.doneCh:
	}
	if s := rq.jw.log.String(); !strings.Contains(s, "wildpanic") {
		t.Error(s)
	}
}

func TestRequest_IncomingRemove(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tss := newTestSetter("")
	rq.UI(NewUiText(tss))

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{What: what.Remove, Data: "Jid.1"}:
	}

	elem := rq.getElementByJid(1)
	for elem != nil {
		select {
		case <-th.C:
			th.Timeout()
		default:
			time.Sleep(time.Millisecond)
			elem = rq.getElementByJid(1)
		}
	}
}

func TestRequest_IncomingClick(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	tjc1 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}
	tjc1.err = ErrEventUnhandled
	tjc2 := &testJawsClick{
		clickCh:    make(chan string, 2),
		testSetter: newTestSetter(""),
	}

	rq.Div("1", tjc1)
	rq.Div("2", tjc2)

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{What: what.Click, Data: "name\tJid.1\tJid.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case s := <-tjc2.clickCh:
		if s != "name" {
			t.Error(s)
		}
	}
	select {
	case s := <-tjc1.clickCh:
		t.Errorf("should have been ignored, got %q", s)
	default:
	}
}

func TestRequest_CustomErrors(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	cause := newErrNoWebSocketRequest(rq.Request)
	err := newErrPendingCancelled(rq.Request, cause)
	th.True(errors.Is(err, ErrPendingCancelled{}))
	th.True(errors.Is(err, ErrNoWebSocketRequest{}))
	th.Equal(errors.Is(cause, ErrPendingCancelled{}), false)
	var target1 ErrNoWebSocketRequest
	th.True(errors.As(err, &target1))
	var target2 ErrPendingCancelled
	th.Equal(errors.As(cause, &target2), false)
}
