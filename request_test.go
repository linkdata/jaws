package jaws

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
)

const testTimeout = time.Second * 3

func fillWsCh(ch chan wsMsg) {
	for {
		select {
		case ch <- wsMsg{}:
		default:
			return
		}
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	x := &testUi{}

	is.Equal(rq.wantMessage(&Message{Dest: x}), false)
	jid := rq.Register(x)
	is.True(jid.IsValid())
	is.Equal(rq.wantMessage(&Message{Dest: x}), true)
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
	is.Equal(strings.Contains(txt, rq.JawsKeyString()), true)
	is.Equal(strings.Contains(txt, JavascriptPath), true)
	is.Equal(strings.Count(txt, "<script>"), strings.Count(txt, "</script>"))
	is.Equal(strings.Count(txt, "<style>"), strings.Count(txt, "</style>"))
}

func TestRequestWriter_TailHTML(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()
	item := &testUi{}
	item.updateFn = func(e *Element) {
		e.SetAttr("hidden", "yes")
		e.RemoveAttr("hidden")
		e.SetClass("cls")
		e.RemoveClass("cls")
	}
	jid := rq.Register(item)
	th.Equal(jid, Jid(1))
	th.Equal(atomic.LoadInt32(&item.updateCalled), int32(1))
	var buf bytes.Buffer
	rq.rq.Writer(&buf).TailHTML()
	want := `
<script>
document.getElementById("Jid.1")?.setAttribute("hidden","yes");
document.getElementById("Jid.1")?.removeAttribute("hidden");
document.getElementById("Jid.1")?.classList?.add("cls");
document.getElementById("Jid.1")?.classList?.remove("cls");
</script>`
	th.Equal(want, buf.String())
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	x := &testUi{}
	jid := rq.Register(x)
	elem := rq.getElementByJid(jid)
	is.True(elem != nil)
	rq.jw.Broadcast(Message{Dest: x, What: what.Inner, Data: "bar"})
	select {
	case <-time.NewTimer(time.Hour).C:
		is.Error("timeout")
	case msg := <-rq.outCh:
		elem := rq.getElementByJid(jid)
		is.True(elem != nil)
		if elem != nil {
			is.Equal(msg, wsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
		}
	}
}

func TestRequest_OutboundRespectsContextDone(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	var callCount int32
	x := &testUi{}
	rq.Register(x, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel()
		return errors.New(val)
	})
	fillWsCh(rq.outCh)
	rq.jw.Broadcast(Message{Dest: x, What: what.Hook, Data: "bar"})

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

func TestRequest_Trigger(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	gotFooCall := make(chan struct{})
	gotEndCall := make(chan struct{})
	fooItem := &testUi{}
	rq.Register(fooItem, func(e *Element, evt what.What, val string) error {
		defer close(gotFooCall)
		return nil
	})
	errItem := &testUi{}
	rq.Register(errItem, func(e *Element, evt what.What, val string) error {
		return errors.New(val)
	})
	endItem := &testUi{}
	rq.Register(endItem, func(e *Element, evt what.What, val string) error {
		defer close(gotEndCall)
		return nil
	})

	// broadcasts from ourselves should not invoke fn
	rq.jw.Broadcast(Message{Dest: endItem, What: what.Input, Data: ""}) // to know when to stop
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
	rq.jw.Broadcast(Message{Dest: fooItem, What: what.Input, Data: "bar"})
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.outCh:
		th.Fatal(s)
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.jw.Broadcast(Message{Dest: errItem, What: what.Input, Data: "omg"})
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		th.Equal(msg.Format(), (&wsMsg{
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
	sleepItem := &testUi{}
	rq.Register(sleepItem, func(e *Element, evt what.What, val string) error {
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
		rq.jw.Broadcast(Message{Dest: sleepItem, What: what.Input, Data: strconv.Itoa(i + 1)})
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

	bombItem := &testUi{}
	rq.Register(bombItem, func(e *Element, evt what.What, val string) error {
		time.Sleep(time.Millisecond * time.Duration(atomic.AddInt32(&wait, 1)))
		return nil
	})

	rq.expectPanic = true
	rq.jw.Logger = nil
	jid := jidForTag(rq.Request, bombItem)

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
	spewItem := &testUi{}
	rq.Register(spewItem, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		if len(rq.outCh) < cap(rq.outCh) {
			rq.jw.Broadcast(Message{Dest: spewItem, What: what.Input})
		} else {
			atomic.StoreInt32(&spewState, 1)
			for atomic.LoadInt32(&spewState) == 1 {
				time.Sleep(time.Millisecond)
			}
		}
		return errors.New("chunks")
	})

	fooItem := &testUi{}
	rq.Register(fooItem)

	rq.jw.Broadcast(Message{Dest: spewItem, What: what.Input})

	// wait for the event fn to be in hold state
	waited := 0
	for waited < 1000 && atomic.LoadInt32(&spewState) == 0 {
		time.Sleep(time.Millisecond)
		waited++
	}
	th.Equal(atomic.LoadInt32(&spewState), int32(1))
	th.Equal(cap(rq.outCh), len(rq.outCh))
	th.True(waited < 1000)

	rq.cancel()

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
	case msg := <-rq1.outCh:
		s := msg.Format()
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
	case msg := <-rq1.outCh:
		s := msg.Format()
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
	case msg := <-rq.outCh:
		s := msg.Format()
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
	case msg := <-rq1.outCh:
		s := msg.Format()
		if s != "Delete\tJid.1\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.outCh:
		s := msg.Format()
		if s != "Delete\tJid.3\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.outCh:
		s := msg.Format()
		if s != "Delete\tJid.4\t\"\"\n" {
			t.Errorf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.outCh:
		s := msg.Format()
		if s != "Delete\tJid.6\t\"\"\n" {
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
	case msg := <-rq1.outCh:
		s := msg.Format()
		if s != "Inner\tfooId\t\"inner\"\n" {
			t.Errorf("%q", s)
		}
	}
	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.outCh:
		s := msg.Format()
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

func TestRequest_Dirty(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	tss1 := &testUi{s: "foo1"}
	tss2 := &testUi{s: "foo2"}
	rq.UI(NewUiText(tss1))
	rq.UI(NewUiText(tss2))
	th.Equal(tss1.getCalled, int32(1))
	th.Equal(tss2.getCalled, int32(1))
	th.True(strings.Contains(string(rq.BodyString()), "foo1"))
	th.True(strings.Contains(string(rq.BodyString()), "foo2"))

	rq.Dirty(tss1)
	rq.Dirty(tss2)
	for atomic.LoadInt32(&tss1.getCalled) < 2 && atomic.LoadInt32(&tss2.getCalled) < 2 {
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
	err := newErrPendingCancelledLocked(rq.Request, cause)
	th.True(errors.Is(err, ErrPendingCancelled))
	th.True(errors.Is(err, ErrNoWebSocketRequest))
	th.Equal(errors.Is(cause, ErrPendingCancelled), false)
	var target1 errNoWebSocketRequest
	th.True(errors.As(err, &target1))
	var target2 errPendingCancelled
	th.Equal(errors.As(cause, &target2), false)
}

func TestRequest_renderDebugLocked(t *testing.T) {
	is := newTestHelper(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer jw.recycle(rq)

	tss := &testUi{}
	e := rq.NewElement(tss)
	e.Tag(Tag("zomg"))

	var sb strings.Builder
	e.renderDebug(&sb)

	txt := sb.String()
	is.Equal(strings.Contains(txt, "zomg"), true)
	is.Equal(strings.Contains(txt, "n/a"), false)

	rq.mu.Lock()
	defer rq.mu.Unlock()
	sb.Reset()
	e.renderDebug(&sb)

	txt = sb.String()
	is.Equal(strings.Contains(txt, "zomg"), false)
	is.Equal(strings.Contains(txt, "n/a"), true)
}
