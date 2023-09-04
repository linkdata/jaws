package jaws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

const testTimeout = time.Second * 3

type testRequest struct {
	is *is.I
	jw *Jaws
	*Request
	log         bytes.Buffer
	readyCh     chan struct{}
	doneCh      chan struct{}
	inCh        chan wsMsg
	outCh       chan wsMsg
	bcastCh     chan Message
	ctx         context.Context
	cancel      context.CancelFunc
	expectPanic bool
	panicked    bool
}

func newTestRequest(is *is.I) (tr *testRequest) {
	tr = &testRequest{
		is:      is,
		readyCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
		jw:      New(),
	}
	tr.jw.Logger = log.New(&tr.log, "", 0)
	tr.ctx, tr.cancel = context.WithTimeout(context.Background(), time.Hour)
	tr.Request = tr.jw.NewRequest(tr.ctx, nil)
	tr.Request.tickerCh = time.NewTicker(time.Millisecond).C

	tr.jw.UseRequest(tr.JawsKey, nil)

	go tr.jw.Serve()

	tr.inCh = make(chan wsMsg)
	tr.bcastCh = tr.Jaws.subscribe(tr.Request, 64)
	tr.outCh = make(chan wsMsg, cap(tr.bcastCh))

	// ensure subscription is processed
	for i := 0; i <= cap(tr.Jaws.subCh); i++ {
		tr.Jaws.subCh <- subscription{}
	}

	go func() {
		defer func() {
			if tr.expectPanic {
				tr.panicked = (recover() != nil)
			}
			close(tr.doneCh)
		}()
		close(tr.readyCh)
		tr.process(tr.bcastCh, tr.inCh, tr.outCh) // usubs from bcase, closes outCh
		tr.recycle()
	}()

	return
}

func (tr *testRequest) Close() {
	tr.cancel()
	tr.jw.Close()
}

func fillWsCh(ch chan wsMsg) {
	for {
		select {
		case ch <- wsMsg{}:
		default:
			return
		}
	}
}

func fillTagCh(ch chan Message) {
	for {
		select {
		case ch <- Message{}:
		default:
			return
		}
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	is.Equal(rq.wantMessage(&Message{Tag: "bar"}), false)

	jid := rq.Register("sometag")
	is.True(jid != 0)
	fn, ok := rq.GetEventFn(jid)
	is.Equal(ok, true)
	is.Equal(fn, nil)

	var ef EventFn = func(rq *Request, evt what.What, id, val string) error {
		return nil
	}
	jid2 := rq.Register(jid, ef)
	is.Equal(jid, jid2)
	fn, ok = rq.GetEventFn(jid)
	is.Equal(ok, true)
	is.Equal(ef, fn)

	jid2 = rq.Register(jid)
	is.Equal(jid, jid2)
	fn, ok = rq.GetEventFn(jid)
	is.Equal(ok, true)
	is.Equal(fn, ef)

	/*rq.SetEventFn(id, nil)
	fn, ok = rq.GetEventFn(id)
	is.Equal(ok, true)
	is.Equal(fn, nil)
	is.Equal(rq.OnEvent(id, ef), nil)*/

	jid = rq.Register("foo")
	is.True(jid != 0)
	jid2 = rq.Register("")
	is.True(jid2 != 0)
	is.True(jid != jid2)
}

func TestRequest_DuplicateRegistration(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), nil)
	var ef1 EventFn = func(rq *Request, evt what.What, id, val string) error { return nil }
	var ef2 EventFn = func(rq *Request, evt what.What, id, val string) error { return errors.New("fails") }
	is.Equal(rq.Register("foo", ef1), Jid(1))  // first reg succeeds
	is.Equal(rq.Register(Jid(1), ef1), Jid(1)) // second reg succeeds
	rq2 := jw.UseRequest(rq.JawsKey, nil)
	is.Equal(rq, rq2)
	is.Equal(rq.Register(Jid(1), ef1), Jid(1))
	// should succeed and not overwrite event fn
	is.Equal(rq.Register(Jid(1), nil), Jid(1))
	ef, ok := rq.GetEventFn(Jid(1))
	is.True(ok)
	is.Equal(ef, ef1)
	// replace the event fn
	is.Equal(rq.Register(Jid(1), ef2), Jid(1))
	ef, ok = rq.GetEventFn(Jid(1))
	is.True(ok)
	is.Equal(ef, ef2)
}

func TestRequest_SendFailsWhenJawsClosed(t *testing.T) {
	is := is.New(t)
	jw := New()
	rq := jw.NewRequest(context.Background(), nil)
	jw.UseRequest(rq.JawsKey, nil)
	jw.Close()
	is.Equal(rq.Send(Message{}), false)
}

func TestRequest_SendPanicsAfterRecycle(t *testing.T) {
	// can not run in parallel
	is := is.New(t)
	defer func() {
		e := recover()
		if e == nil {
			is.Fail()
		}
		t.Log(e)
	}()
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), nil)
	rq.recycle()
	rq.Send(Message{})
}

func TestRequest_SendFailsWhenContextDone(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(ctx, nil)
	jw.UseRequest(rq.JawsKey, nil)
	defer rq.recycle()
	fillTagCh(rq.sendCh)
	cancel()
	is.Equal(rq.Send(Message{}), false)
}

func TestRequest_HeadHTML(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), nil)
	defer rq.recycle()

	txt := rq.HeadHTML()
	is.Equal(strings.Contains(string(txt), rq.JawsKeyString()), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
}

func TestRequest_BroadcastsCallable(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.Serve()
	rq := jw.NewRequest(context.Background(), nil)
	defer rq.recycle()

	// TODO
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	rq.Register(Tag{"foo"})
	theMsg := Message{Tag: "foo", What: what.Inner, Data: "bar"}

	is.Equal(rq.Send(theMsg), true)
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case msg := <-rq.outCh:
		elem := rq.GetElement(msg.Jid)
		is.True(elem != nil)
		is.Equal(msg, wsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
	}
}

func TestRequest_OutboundRespectsJawsClosed(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	jw := rq.jw
	var callCount int32
	tag := Tag{"foo"}
	rq.RegisterEventFn(tag, func(rq *Request, evt what.What, id, val string) error {
		atomic.AddInt32(&callCount, 1)
		is.Equal(1, jw.RequestCount())
		jw.Close()
		return errors.New(val)
	})
	fillWsCh(rq.outCh)
	jw.Broadcast(Message{Tag: "foo", What: what.Hook, Data: "bar"})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Equal(int(atomic.LoadInt32(&callCount)), 0)
		is.Fail()
	case <-jw.Done():
	}
	is.Equal(int(atomic.LoadInt32(&callCount)), 1)
}

func TestRequest_OutboundRespectsContextDone(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	var callCount int32
	tag := Tag{"foo"}
	rq.RegisterEventFn(tag, func(_ *Request, evt what.What, id, val string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel()
		return errors.New(val)
	})
	fillWsCh(rq.outCh)
	rq.jw.Broadcast(Message{Tag: "foo", What: what.Hook, Data: "bar"})

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Equal(int(atomic.LoadInt32(&callCount)), 0)
		is.Fail()
	case <-rq.jw.Done():
		is.Fail()
	case <-rq.ctx.Done():
	}

	is.Equal(int(atomic.LoadInt32(&callCount)), 1)
}

func TestRequest_OutboundOverflowPanicsWithNoLogger(t *testing.T) {
	// can not run in parallel
	is := is.New(t)
	rq := newTestRequest(is)
	rq.expectPanic = true
	rq.jw.Logger = nil
	defer rq.Close()
	rq.Register(Tag{"foo"})
	fillWsCh(rq.outCh)
	rq.sendCh <- Message{Tag: "foo", What: what.Inner, Data: "bar"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.doneCh:
		is.Equal(len(rq.outCh), cap(rq.outCh))
		is.True(rq.panicked)
	}
}

func TestRequest_Trigger(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	gotFooCall := make(chan struct{})
	gotEndCall := make(chan struct{})
	rq.RegisterEventFn(Tag{"foo"}, func(rq *Request, evt what.What, id, val string) error {
		defer close(gotFooCall)
		return nil
	})
	rq.RegisterEventFn(Tag{"err"}, func(rq *Request, evt what.What, id, val string) error {
		return errors.New(val)
	})
	rq.RegisterEventFn(Tag{"end"}, func(rq *Request, evt what.What, id, val string) error {
		defer close(gotEndCall)
		return nil
	})

	// broadcasts from ourselves should not invoke fn
	rq.Trigger("foo", "bar")
	rq.Trigger("err", "baz")
	rq.jw.Trigger("end", "") // to know when to stop
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.outCh:
		is.Fail()
	case <-gotFooCall:
		is.Fail()
	case <-gotEndCall:
	}

	// global broadcast should invoke fn
	rq.jw.Trigger("foo", "bar")
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.outCh:
		is.Fail()
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.jw.Trigger("err", "omg")
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg.Jid, Jid(0))
		is.Equal(msg.What, what.Alert)
		is.Equal(msg.Data, "danger\nomg")
	}
}

func TestRequest_EventFnQueue(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	// calls to slow event functions queue up and are executed in order
	firstDoneCh := make(chan struct{})
	var sleepDone int32
	var callCount int32
	rq.RegisterEventFn(Tag{"sleep"}, func(rq *Request, evt what.What, id, val string) error {
		count := int(atomic.AddInt32(&callCount, 1))
		is.Equal(val, strconv.Itoa(count))
		if count == 1 {
			close(firstDoneCh)
		}
		for atomic.LoadInt32(&sleepDone) == 0 {
			time.Sleep(time.Millisecond)
		}
		return nil
	})

	for i := 0; i < cap(rq.outCh); i++ {
		rq.jw.Trigger("sleep", strconv.Itoa(i+1))
	}

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.doneCh:
		is.Fail()
	case <-firstDoneCh:
	}

	is.Equal(atomic.LoadInt32(&callCount), int32(1))
	atomic.StoreInt32(&sleepDone, 1)

	tmr := time.NewTimer(testTimeout)
	for int(atomic.LoadInt32(&callCount)) < cap(rq.outCh) {
		select {
		case <-tmr.C:
			is.Fail()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	is.Equal(atomic.LoadInt32(&callCount), int32(cap(rq.outCh)))
}

func TestRequest_EventFnQueueOverflowPanicsWithNoLogger(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var wait int32

	rq.RegisterEventFn(Tag{"bomb"}, func(_ *Request, evt what.What, id, val string) error {
		time.Sleep(time.Millisecond * time.Duration(atomic.AddInt32(&wait, 1)))
		return nil
	})

	rq.expectPanic = true
	rq.jw.Logger = nil
	tmr := time.NewTimer(testTimeout)
	defer tmr.Stop()
	for {
		select {
		case rq.sendCh <- Message{Tag: "bomb", What: what.Trigger}:
		case <-rq.doneCh:
			is.True(rq.panicked)
			return
		case <-tmr.C:
			is.Fail()
		}
	}
}

func TestRequest_IgnoresIncomingMsgsDuringShutdown(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var spewState int32
	var callCount int32
	rq.RegisterEventFn(Tag{"spew"}, func(_ *Request, evt what.What, id, val string) error {
		atomic.AddInt32(&callCount, 1)
		if len(rq.outCh) < cap(rq.outCh) {
			rq.jw.Trigger("spew", "")
		} else {
			atomic.StoreInt32(&spewState, 1)
			for atomic.LoadInt32(&spewState) == 1 {
				time.Sleep(time.Millisecond)
			}
		}
		return errors.New("chunks")
	})
	rq.Register(Tag{"foo"})

	rq.jw.Trigger("spew", "")

	// wait for the event fn to be in hold state
	waited := 0
	for waited < 1000 && atomic.LoadInt32(&spewState) == 0 {
		time.Sleep(time.Millisecond)
		waited++
	}
	is.Equal(atomic.LoadInt32(&spewState), int32(1))
	is.Equal(cap(rq.outCh), len(rq.outCh))
	is.True(waited < 1000)

	// sending a message will now fail the rq since the
	// outbound channel is full, but with the
	// event fn holding it won't be able to end
	select {
	case rq.sendCh <- Message{Tag: "foo", What: what.Inner, Data: ""}:
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.doneCh:
		is.Fail()
	}

	// rq should now be in shutdown phase draining channels
	// while waiting for the event fn to return
	tmr := time.NewTimer(testTimeout)
	for i := 0; i < cap(rq.outCh)*2; i++ {
		select {
		case rq.sendCh <- Message{}:
		case <-rq.doneCh:
			is.Fail()
		case <-tmr.C:
			is.Fail()
		}
		select {
		case rq.inCh <- wsMsg{}:
		case <-rq.doneCh:
			is.Fail()
		case <-tmr.C:
			is.Fail()
		}
	}

	// release the event fn
	atomic.StoreInt32(&spewState, 2)

	select {
	case <-rq.doneCh:
		is.True(atomic.LoadInt32(&callCount) > 1)
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	}

	// log data should contain message that we were unable to deliver error
	is.True(strings.Contains(rq.log.String(), "outboundMsgCh full sending event"))
}

func TestRequest_Sends(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	rq.Register(Tag{"SetAttr"})
	setAttrElement := rq.GetElements("SetAttr")[0]

	rq.Register(Tag{"RemoveAttr"})
	removeAttrElement := rq.GetElements("RemoveAttr")[0]

	gotSetAttr := ""
	gotRemoveAttr := ""
	gotInfoAlert := ""
	gotDangerAlert := ""
	gotRedirect := ""

	is.True(cap(rq.outCh)-len(rq.outCh) > 7)

	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.readyCh:
	}

	setAttrElement.SetAttr("bar", "baz")
	setAttrElement.SetAttr("bar", "baz")
	removeAttrElement.RemoveAttr("bar")

	rq.Alert("info", "<html>\nnot-escaped")
	rq.AlertError(errors.New("<html>\nshould-be-escaped"))
	rq.Redirect("some-url")

	notDone := true
	for notDone {
		select {
		case <-time.NewTimer(testTimeout).C:
			t.Log("timeout")
			notDone = false
		case msg, ok := <-rq.outCh:
			if ok {
				switch rq.GetElement(msg.Jid) {
				case setAttrElement:
					gotSetAttr = msg.Format()
				case removeAttrElement:
					gotRemoveAttr = msg.Format()
				default:
					switch msg.What {
					case what.Alert:
						if strings.HasPrefix(msg.Data, "info\n") {
							gotInfoAlert = msg.Format()
						}
						if strings.HasPrefix(msg.Data, "danger\n") {
							gotDangerAlert = msg.Format()
						}
					case what.Redirect:
						gotRedirect = msg.Format()
						rq.cancel()
					default:
						t.Log(msg)
						t.FailNow()
					}
				}
			}
		case <-rq.doneCh:
			notDone = false
		}
	}

	if !strings.HasSuffix(gotSetAttr, "\nbar\nbaz") {
		t.Log(strconv.Quote(gotSetAttr))
		is.Fail()
	}
	is.True(strings.HasSuffix(gotRemoveAttr, "\nRAttr\nbar"))
	is.Equal(gotRedirect, "0\nRedirect\nsome-url")
	is.Equal(gotInfoAlert, "0\nAlert\ninfo\n<html>\nnot-escaped")
	is.Equal(gotDangerAlert, "0\nAlert\ndanger\n&lt;html&gt;\nshould-be-escaped")
}

/*
func TestRequest_OnInput(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	is.NoErr(rq.OnInput(elemId, func(rq *Request, jid, val string) error {
		defer close(gotCall)
		is.True(rq.GetElement(jid) != nil)
		is.Equal(val, elemVal)
		return nil
	}))
	rq.inCh <- Message{Elem: elemId, What: what.Input, Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_OnClick(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	is.NoErr(rq.OnClick(elemId, func(rq *Request, jid string) error {
		defer close(gotCall)
		is.True(rq.GetElement(jid) != nil)
		return nil
	}))
	rq.inCh <- Message{Elem: elemId, What: what.Click, Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}
*/

func TestRequest_OnTrigger(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	is.NoErr(rq.OnTrigger(elemId, func(rq *Request, jidstr string) error {
		defer close(gotCall)
		is.True(rq.wantMessage(&Message{Tag: elemId}))
		jid := ParseJid(jidstr)
		is.True(jid != 0)
		elem := rq.GetElement(jid)
		is.True(elem != nil)
		return nil
	}))
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Trigger, Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func checkHtml(is *is.I, rq *testRequest, h template.HTML, tag interface{}, txt string) {
	is.Helper()
	if rq.log.Len() > 0 {
		fmt.Println(rq.log.String())
		is.Fail()
	}
	hs := string(h)
	found := false
	elems := rq.GetElements(tag)
	for _, elem := range elems {
		if strings.Contains(hs, elem.Jid().String()) && strings.Contains(hs, txt) {
			found = true
		}
	}
	if !found {
		if len(elems) == 0 {
			elems = rq.elems
		}
		fmt.Printf("checkHtml(%q, %v@%p, %q) did not match any of %d elements:\n", hs, tag, tag, txt, len(elems))
		for i, elem := range elems {
			fmt.Printf("  %d: (%T) id=%q tags=%v data=%v\n", i, elem.UI(), elem.Jid(), elem.Tags(), elem.Data)
		}
		is.Fail()
	}
}

func TestRequest_Elements(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	var avs []*atomic.Value
	for i := 0; i < 16; i++ {
		av := &atomic.Value{}
		av.Store(fmt.Sprintf("t%d", i))
		avs = append(avs, av)
	}

	chk(rq.Div(avs[1], "s1"), avs[1], "s1")
	chk(rq.Span(avs[2], "s2"), avs[2], "s2")
	chk(rq.Li(avs[3], "s3"), avs[3], "s3")
	chk(rq.Td(avs[4], "s4"), avs[4], "s4")
	chk(rq.A(avs[5], "s5"), avs[5], "s5")
	chk(rq.Button(avs[6], "s6"), avs[6], "s6")
	avs[7].Store("randomimg.png")
	chk(rq.Img(avs[7]), avs[7], "src=\"randomimg.png\"")
	avs[8].Store("\"randomimg.png\"")
	chk(rq.Img(avs[8]), avs[8], "src=\"randomimg.png\"")
}

func TestRequest_Text(t *testing.T) {
	const elemVal = "elem-val"
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Text(&av, func(rq *Request, jidstr, val string) error {
		defer close(gotCall)
		is.True(rq.GetElement(ParseJid(jidstr)) != nil)
		is.True(rq.GetElement(ParseJid(jidstr)) != nil)
		is.Equal(val, "other-stuff")
		return nil
	}, "disabled")
	chk(h, &av, elemVal)
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, &av), What: what.Input, Data: "other-stuff"}
	select {
	case <-time.NewTimer(testTimeout).C:
		t.Log(h)
		t.Log(jidForTag(rq.Request, &av))
		is.Fail()
	case <-gotCall:
	}
}

func jidForTag(rq *Request, tag interface{}) Jid {
	if elems := rq.GetElements(tag); len(elems) > 0 {
		return elems[0].jid
	}
	return 0
}

func TestRequest_Password(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store("")

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Password(&av, func(rq *Request, jid, val string) error {
		defer close(gotCall)
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		is.Equal(val, "other-stuff")
		return nil
	}, "autocomplete=\"off\"")
	chk(h, &av, "autocomplete")
	is.True(!strings.Contains(string(h), "value"))
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, &av), What: what.Input, Data: "other-stuff"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Number(t *testing.T) {
	const elemVal = 21.5
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	defer close(gotCall)
	h := rq.Number(elemId, &av, func(rq *Request, jid string, val float64) error {
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		switch val {
		case 4.3:
			// ok
		case 0:
			// ok
		default:
			is.Fail()
		}
		gotCall <- struct{}{}
		return nil
	}, "disabled")
	chk(h, elemId, "21.5")
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "4.3"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: ""} // should call with zero
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "meh"} // should fail with alert
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg.What, what.Alert)
	}
}

func TestRequest_Range(t *testing.T) {
	const elemVal = float64(3.14)
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Range(elemId, &av, func(rq *Request, jid string, val float64) error {
		defer close(gotCall)
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		is.Equal(val, 3.15)
		return nil
	}, "disabled")
	chk(h, elemId, "3.14")
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "3.15"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Checkbox(t *testing.T) {
	const elemVal = true
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	defer close(gotCall)
	h := rq.Checkbox(&av, func(rq *Request, jid string, val bool) error {
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		is.Equal(val, false)
		gotCall <- struct{}{}
		return nil
	})
	chk(h, &av, "checked")
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "false"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: ""}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "wut"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg.What, what.Alert)
	}
}

func TestRequest_Date(t *testing.T) {
	var elemVal time.Time
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	defer close(gotCall)
	h := rq.Date(elemId, &av, func(rq *Request, jid string, val time.Time) error {
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		if !val.IsZero() {
			is.Equal(val.Year(), 1970)
			is.Equal(val.Month(), time.January)
			is.Equal(val.Day(), 2)
		}
		gotCall <- struct{}{}
		return nil
	}, "")

	chk(h, elemId, time.Now().Format(ISO8601))
	is.Equal(av.Load().(time.Time).Round(time.Second), time.Now().Round(time.Second))
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "1970-01-02"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: ""}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "foobar!"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg.What, what.Alert)
	}
}

func TestRequest_Radio(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	var av atomic.Value
	av.Store(true)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Radio(elemId, &av, func(rq *Request, jid string, val bool) error {
		defer close(gotCall)
		is.True(rq.GetElement(ParseJid(jid)) != nil)
		is.Equal(val, false)
		return nil
	})

	chk(h, elemId, "checked")

	rq.inCh <- wsMsg{Jid: jidForTag(rq.Request, elemId), What: what.Input, Data: "false"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Select(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	a := NewNamedBoolArray()
	a.Add("1", "one")
	a.Add("2", "two")

	h := rq.Select(a, "disabled")
	chk(h, a, "disabled")
	is.Equal(strings.Contains(string(h), "selected"), false)

	a.Set("1", true)
	h = rq.Select(a)
	chk(h, a, "selected")
}

func TestRequest_ConnectFn(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	is.Equal(rq.GetConnectFn(), nil)
	is.NoErr(rq.onConnect())

	wantErr := errors.New("getouttahere")
	fn := func(rq *Request) error {
		return wantErr
	}
	rq.SetConnectFn(fn)
	is.Equal(rq.onConnect(), wantErr)
}
