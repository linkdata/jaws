package jaws

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
	inCh        chan *Message
	outCh       chan *Message
	bcastCh     chan *Message
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
	tr.Request = tr.jw.NewRequest(tr.ctx, "")
	tr.jw.UseRequest(tr.JawsKey, "")

	go tr.jw.Serve()

	queueSize := 16 + tr.defaultChSize() // plus 16 since we add elements after creating request
	tr.inCh = make(chan *Message)
	tr.outCh = make(chan *Message, queueSize)
	tr.bcastCh = tr.Jaws.subscribe(queueSize)

	// ensure subscription is processed
	for i := 0; i <= cap(tr.Jaws.subCh); i++ {
		tr.Jaws.subCh <- nil
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

func fillCh(ch chan *Message) {
	for {
		select {
		case ch <- &Message{}:
		default:
			return
		}
	}
}

func TestRequest_Registrations(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	fn, ok := rq.GetEventFn("bar")
	is.Equal(ok, false)
	is.Equal(fn, nil)

	id := rq.Register("")
	is.True(strings.HasPrefix(id, "jaws."))
	fn, ok = rq.GetEventFn(id)
	is.Equal(ok, true)
	is.Equal(fn, nil)

	var ef EventFn = func(rq *Request, id, evt, val string) error {
		return nil
	}
	id2 := rq.RegisterEventFn(id, ef)
	is.Equal(id, id2)
	fn, ok = rq.GetEventFn(id)
	is.Equal(ok, true)
	is.Equal(ef, fn)

	id2 = rq.Register(id)
	is.Equal(id, id2)
	fn, ok = rq.GetEventFn(id)
	is.Equal(ok, true)
	is.Equal(fn, ef)

	rq.SetEventFn(id, nil)
	fn, ok = rq.GetEventFn(id)
	is.Equal(ok, true)
	is.Equal(fn, nil)
	is.Equal(rq.OnEvent(id, ef), nil)

	id = rq.Register("foo")
	is.Equal(id, "foo")
	id2 = rq.Register("")
	is.True(strings.HasPrefix(id2, "jaws."))
	is.True(id != id2)
}

func registerFooShouldPanic(is *is.I, rq *Request) {
	defer func() {
		if v := recover(); v == nil {
			is.Fail()
		} else {
			is.True(strings.Contains(v.(string), "foo"))
		}
	}()
	rq.RegisterEventFn("foo", func(rq *Request, id, evt, val string) error { return nil })
	is.Fail()
}

func TestRequest_DuplicateRegistration(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), "")
	var ef EventFn = func(rq *Request, id, evt, val string) error { return nil }
	is.Equal(rq.RegisterEventFn("foo", ef), "foo")  // first reg succeeds
	is.Equal(rq.RegisterEventFn("foo", nil), "foo") // nil fn always succeeds
	registerFooShouldPanic(is, rq)
	rq2 := jw.UseRequest(rq.JawsKey, "")
	is.Equal(rq, rq2)
	is.Equal(rq.RegisterEventFn("foo", ef), "foo")  // succeeds now that UseRequest() has been called
	is.Equal(rq.RegisterEventFn("foo", nil), "foo") // nil fn always succeeds
}

func TestRequest_SendFailsWhenJawsClosed(t *testing.T) {
	is := is.New(t)
	jw := New()
	rq := jw.NewRequest(context.Background(), "")
	jw.Close()
	is.Equal(rq.Send(&Message{}), false)
}

func TestRequest_SendPanicsAfterRecycle(t *testing.T) {
	// can not run in parallel
	is := is.New(t)
	defer func() {
		if recover() == nil {
			is.Fail()
		}
	}()
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), "")
	rq.recycle()
	rq.Send(&Message{})
}

func TestRequest_SendFailsWhenContextDone(t *testing.T) {
	is := is.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(ctx, "")
	defer rq.recycle()
	fillCh(rq.sendCh)
	cancel()
	is.Equal(rq.Send(&Message{}), false)
}

func TestRequest_HeadHTML(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), "")
	defer rq.recycle()

	txt := rq.HeadHTML()
	is.Equal(strings.Contains(string(txt), rq.JawsKeyString()), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
}

func TestRequest_BroadcastsCallable(t *testing.T) {
	jw := New()
	defer jw.Close()
	go jw.Serve()
	rq := jw.NewRequest(context.Background(), "")
	defer rq.recycle()

	rq.SetInner("foo", "bar")
	rq.SetTextValue("foo", "bar")
	rq.SetFloatValue("foo", 1.1)
	rq.SetBoolValue("foo", true)
	rq.SetDateValue("foo", time.Now())
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	rq.Register("foo")
	theMsg := &Message{Elem: "foo"}
	is.Equal(rq.Send(theMsg), true)
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg, theMsg)
	}
}

func TestRequest_OutboundRespectsJawsClosed(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()
	jw := rq.jw
	var callCount int32
	rq.RegisterEventFn("foo", func(rq *Request, id, evt, val string) error {
		atomic.AddInt32(&callCount, 1)
		jw.Close()
		return errors.New(val)
	})
	fillCh(rq.outCh)
	jw.Broadcast(&Message{Elem: "foo", What: "hook", Data: "bar"})
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
	rq.RegisterEventFn("foo", func(_ *Request, id, evt, val string) error {
		atomic.AddInt32(&callCount, 1)
		rq.cancel()
		return errors.New(val)
	})
	fillCh(rq.outCh)
	rq.jw.Broadcast(&Message{Elem: "foo", What: "hook", Data: "bar"})

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
	rq.Register("foo")
	fillCh(rq.outCh)
	rq.sendCh <- &Message{Elem: "foo", What: "", Data: "bar"}
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
	rq.RegisterEventFn("foo", func(rq *Request, id, evt, val string) error {
		defer close(gotFooCall)
		return nil
	})
	rq.RegisterEventFn("err", func(rq *Request, id, evt, val string) error {
		return errors.New(val)
	})
	rq.RegisterEventFn("end", func(rq *Request, id, evt, val string) error {
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
		is.Equal(msg.Elem, " alert")
		is.Equal(msg.What, "danger")
		is.Equal(msg.Data, "omg")
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
	rq.RegisterEventFn("sleep", func(rq *Request, id, evt, val string) error {
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

	rq.RegisterEventFn("bomb", func(_ *Request, id, evt, val string) error {
		time.Sleep(time.Millisecond)
		return nil
	})

	rq.expectPanic = true
	rq.jw.Logger = nil
	tmr := time.NewTimer(testTimeout)
	for {
		select {
		case rq.sendCh <- &Message{Elem: "bomb", What: "trigger"}:
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
	rq.RegisterEventFn("spew", func(_ *Request, id, evt, val string) error {
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
	rq.Register("foo")

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
	case rq.sendCh <- &Message{Elem: "foo"}:
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
		case rq.sendCh <- &Message{}:
		case <-rq.doneCh:
			is.Fail()
		case <-tmr.C:
			is.Fail()
		}
		select {
		case rq.inCh <- &Message{}:
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

	rq.Register("SetAttr")
	rq.Register("RemoveAttr")

	gotSetAttr := ""
	gotRemoveAttr := ""
	gotInfoAlert := ""
	gotDangerAlert := ""
	gotRedirect := ""

	is.True(cap(rq.outCh)-len(rq.outCh) > 7)

	<-rq.readyCh

	rq.SetAttr("SetAttr", "bar", "baz")
	rq.SetAttr("NotRegistered", "bar", "baz")
	rq.RemoveAttr("RemoveAttr", "bar")
	rq.Alert("info", "<html>\nnot-escaped")
	rq.AlertError(errors.New("<html>\nshould-be-escaped"))
	rq.Redirect("some-url")

	for msg := range rq.outCh {
		switch msg.Elem {
		case "SetAttr":
			gotSetAttr = msg.Format()
		case "RemoveAttr":
			gotRemoveAttr = msg.Format()
		case "NotRegistered":
			is.Fail()
		case " alert":
			switch msg.What {
			case "info":
				gotInfoAlert = msg.Format()
			case "danger":
				gotDangerAlert = msg.Format()
			}
		case " redirect":
			gotRedirect = msg.Format()
			rq.cancel()
		}
	}

	<-rq.doneCh

	is.Equal(gotSetAttr, "SetAttr\nsattr\nbar\nbaz")
	is.Equal(gotRemoveAttr, "RemoveAttr\nrattr\nbar")
	is.Equal(gotInfoAlert, " alert\ninfo\n<html>\nnot-escaped")
	is.Equal(gotDangerAlert, " alert\ndanger\n&lt;html&gt;\nshould-be-escaped")
	is.Equal(gotRedirect, " redirect\nsome-url\n")
}

func TestRequest_OnInput(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	gotCall := make(chan struct{})
	rq := newTestRequest(is)
	defer rq.Close()
	is.NoErr(rq.OnInput(elemId, func(rq *Request, val string) error {
		defer close(gotCall)
		is.Equal(val, elemVal)
		return nil
	}))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: elemVal}
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
	is.NoErr(rq.OnClick(elemId, func(rq *Request) error {
		defer close(gotCall)
		return nil
	}))
	rq.inCh <- &Message{Elem: elemId, What: "click", Data: elemVal}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Elements(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	h := rq.Div(elemId, elemVal, nil, "")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Span(elemId, elemVal, nil, "")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Li(elemId, elemVal, nil, "")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Td(elemId, elemVal, nil, "")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.A(elemId, elemVal, nil, "")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

	h = rq.Button(elemId, elemVal, nil, "disabled")
	is.True(strings.Contains(string(h), elemId))
	is.True(strings.Contains(string(h), elemVal))

}

func TestRequest_Text(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = "elem-val"
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Text(elemId, elemVal, func(rq *Request, val string) error {
		defer close(gotCall)
		is.Equal(val, "other-stuff")
		return nil
	}, "disabled")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), elemVal))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "other-stuff"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Password(t *testing.T) {
	const elemId = "elem-id"
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Password(elemId, func(rq *Request, val string) error {
		defer close(gotCall)
		is.Equal(val, "other-stuff")
		return nil
	}, "autocomplete=\"off\"")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(!strings.Contains(string(h), "value"))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "other-stuff"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Number(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = 21.5
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Number(elemId, elemVal, func(rq *Request, val float64) error {
		defer close(gotCall)
		is.Equal(val, 4.3)
		return nil
	}, "disabled")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), "21.5"))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "4.3"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Range(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = float64(3.14)
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Range(elemId, elemVal, func(rq *Request, val float64) error {
		defer close(gotCall)
		is.Equal(val, 3.15)
		return nil
	}, "disabled")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), "3.14"))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "3.15"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Checkbox(t *testing.T) {
	const elemId = "elem-id"
	const elemVal = true
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Checkbox(elemId, elemVal, func(rq *Request, val bool) error {
		defer close(gotCall)
		is.Equal(val, false)
		return nil
	}, "")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), "checked"))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "false"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Date(t *testing.T) {
	const elemId = "elem-id"
	var elemVal time.Time
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Date(elemId, elemVal, func(rq *Request, val time.Time) error {
		defer close(gotCall)
		is.Equal(val.Year(), 1970)
		is.Equal(val.Month(), time.January)
		is.Equal(val.Day(), 2)
		return nil
	}, "")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), time.Now().Format(ISO8601)))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "1970-01-02"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_Radio(t *testing.T) {
	const elemId = "buttonid/groupid"
	const elemVal = true
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	gotCall := make(chan struct{})
	h := rq.Radio(elemId, elemVal, func(rq *Request, val bool) error {
		defer close(gotCall)
		is.Equal(val, false)
		return nil
	}, "")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.True(strings.Contains(string(h), "name=\"groupid\""))
	is.True(strings.Contains(string(h), "checked"))
	rq.inCh <- &Message{Elem: elemId, What: "input", Data: "false"}
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-gotCall:
	}
}

func TestRequest_RadioPanicOnInvalidID(t *testing.T) {
	is := is.New(t)
	defer func() {
		if recover() == nil {
			is.Fail()
		}
	}()
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(context.Background(), "")
	rq.Radio("missinggroup", true, nil, "")
}

func TestRequest_Select(t *testing.T) {
	const elemId = "elemid"
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	a := NewNamedBoolArray()
	a.Add("1", "one")
	a.Add("2", "two")

	h := rq.Select(elemId, a, nil, "disabled")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.Equal(strings.Contains(string(h), "selected"), false)

	a.Check("1")
	h = rq.Select(elemId, a, nil, "")
	is.True(strings.Contains(string(h), "id=\""+elemId+"\""))
	is.Equal(strings.Contains(string(h), "selected"), true)
}

func TestRequest_ConnectFn(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest(is)
	defer rq.Close()

	is.NoErr(rq.onConnect())

	wantErr := errors.New("getouttahere")
	rq.ConnectFn = func(rq *Request) error {
		return wantErr
	}

	is.Equal(rq.onConnect(), wantErr)
}
