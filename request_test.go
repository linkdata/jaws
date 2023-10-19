package jaws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/linkdata/jaws/jid"
	"github.com/linkdata/jaws/what"
	"github.com/matryer/is"
)

const testTimeout = time.Second * 3

type testRequest struct {
	jw *Jaws
	*Request
	log         bytes.Buffer
	readyCh     chan struct{}
	doneCh      chan struct{}
	inCh        chan wsMsg
	outCh       chan string
	bcastCh     chan Message
	ctx         context.Context
	cancel      context.CancelFunc
	expectPanic bool
	panicked    bool
	panicVal    any
}

func newTestRequest() (tr *testRequest) {
	tr = &testRequest{
		readyCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
		jw:      New(),
	}
	tr.jw.Logger = log.New(&tr.log, "", 0)
	tr.jw.Template = template.Must(template.New("testtemplate").Parse(`{{with $.Dot}}<div id="{{$.Jid}}"{{$.Attrs}}>{{.}}</div>{{end}}`))
	tr.jw.updateTicker = time.NewTicker(time.Millisecond)
	tr.ctx, tr.cancel = context.WithTimeout(context.Background(), time.Hour)
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	tr.Request = tr.jw.NewRequest(hr)

	tr.jw.UseRequest(tr.JawsKey, hr.WithContext(tr.ctx))

	go tr.jw.Serve()

	tr.inCh = make(chan wsMsg)
	tr.bcastCh = tr.Jaws.subscribe(tr.Request, 64)
	tr.outCh = make(chan string, cap(tr.bcastCh))

	// ensure subscription is processed
	for i := 0; i <= cap(tr.Jaws.subCh); i++ {
		tr.Jaws.subCh <- subscription{}
	}

	go func() {
		defer func() {
			if tr.expectPanic {
				if tr.panicVal = recover(); tr.panicVal != nil {
					tr.panicked = true
				}
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

func fillWsCh(ch chan string) {
	for {
		select {
		case ch <- "":
		default:
			return
		}
	}
}

/*func fillTagCh(ch chan Message) {
	for {
		select {
		case ch <- Message{}:
		default:
			return
		}
	}
}*/

func TestRequest_Registrations(t *testing.T) {
	is := is.New(t)
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

/*func TestRequest_SendFailsWhenJawsClosed(t *testing.T) {
	is := is.New(t)
	jw := New()
	rq := jw.NewRequest(nil)
	jw.UseRequest(rq.JawsKey, nil)
	jw.Close()
	is.Equal(rq.send(Message{}), false)
}*/

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
	rq := jw.NewRequest(nil)
	rq.recycle()
	rq.Jaws.Broadcast(Message{})
}

/*func TestRequest_SendFailsWhenContextDone(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	hr := httptest.NewRequest(http.MethodGet, "/", nil)
	rq := jw.NewRequest(hr)
	ctx, cancel := context.WithCancel(context.Background())
	jw.UseRequest(rq.JawsKey, hr.WithContext(ctx))
	defer rq.recycle()
	if rq.cancelFn == nil {
		is.Fail()
	}
	cancel()
	is.Equal(rq.send(Message{}), false)
}*/

func TestRequest_HeadHTML(t *testing.T) {
	is := is.New(t)
	jw := New()
	defer jw.Close()
	rq := jw.NewRequest(nil)
	defer rq.recycle()

	txt := rq.HeadHTML()
	is.Equal(strings.Contains(string(txt), rq.JawsKeyString()), true)
	is.Equal(strings.Contains(string(txt), JavascriptPath), true)
}

func TestRequest_SendArrivesOk(t *testing.T) {
	is := is.New(t)
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
		elem := rq.GetElement(jid)
		is.True(elem != nil)
		is.Equal(msg, wsMsg{Jid: elem.jid, Data: "bar", What: what.Inner})
	}
}

func TestRequest_OutboundRespectsJawsClosed(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()
	jw := rq.jw
	var callCount int32
	tag := Tag("foo")
	rq.Register(tag, func(e *Element, evt what.What, val string) error {
		atomic.AddInt32(&callCount, 1)
		is.Equal(1, jw.RequestCount())
		jw.Close()
		return errors.New(val)
	})
	fillWsCh(rq.outCh)
	jw.Broadcast(Message{Dest: Tag("foo"), What: what.Hook, Data: "bar"})
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
	rq := newTestRequest()
	rq.expectPanic = true
	rq.jw.Logger = nil
	defer rq.Close()
	rq.Register(Tag("foo"))
	fillWsCh(rq.outCh)
	rq.Jaws.Broadcast(Message{Dest: Tag("foo"), What: what.Inner, Data: "bar"})
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.outCh:
		is.Fail()
	case <-gotFooCall:
		is.Fail()
	case <-gotEndCall:
	}

	// global broadcast should invoke fn
	rq.jw.Broadcast(Message{Dest: Tag("foo"), What: what.Input, Data: "bar"})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.outCh:
		is.Fail()
	case <-gotFooCall:
	}

	// fn returning error should send an danger alert message
	rq.jw.Broadcast(Message{Dest: Tag("err"), What: what.Input, Data: "omg"})
	select {
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case msg := <-rq.outCh:
		is.Equal(msg, (&wsMsg{
			Data: "danger\nomg",
			Jid:  jid.Jid(0),
			What: what.Alert,
		}).Format())
	}
}

func TestRequest_EventFnQueue(t *testing.T) {
	is := is.New(t)
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
			is.Fail()
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
	case <-time.NewTimer(testTimeout * 100).C:
		is.Fail()
	case <-rq.doneCh:
		is.Fail()
	case <-firstDoneCh:
	}

	is.Equal(atomic.LoadInt32(&callCount), int32(1))
	atomic.StoreInt32(&sleepDone, 1)
	is.Equal(rq.panicVal, nil)

	tmr := time.NewTimer(testTimeout * 100)
	for int(atomic.LoadInt32(&callCount)) < cap(rq.outCh) {
		select {
		case <-tmr.C:
			t.Logf("callCount=%d, cap=%d", atomic.LoadInt32(&callCount), cap(rq.outCh))
			is.Equal(rq.panicVal, nil)
			is.Fail()
		default:
			time.Sleep(time.Millisecond)
		}
	}
	is.Equal(atomic.LoadInt32(&callCount), int32(cap(rq.outCh)))
}

func TestRequest_EventFnQueueOverflowPanicsWithNoLogger(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()

	var wait int32

	rq.Register(("bomb"), func(e *Element, evt what.What, val string) error {
		time.Sleep(time.Millisecond * time.Duration(atomic.AddInt32(&wait, 1)))
		return nil
	})

	rq.expectPanic = true
	rq.jw.Logger = nil
	tmr := time.NewTimer(testTimeout)

	jid := jidForTag(rq.Request, Tag("bomb"))

	defer tmr.Stop()
	for {
		select {
		case <-rq.doneCh:
			is.True(rq.panicked)
			is.True(strings.Contains(rq.panicVal.(error).Error(), "eventCallCh is full sending"))
			return
		case <-tmr.C:
			is.Fail()
		case rq.inCh <- wsMsg{Jid: jid, What: what.Input}:
		}
	}
}

func TestRequest_IgnoresIncomingMsgsDuringShutdown(t *testing.T) {
	is := is.New(t)
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
	is.Equal(atomic.LoadInt32(&spewState), int32(1))
	is.Equal(cap(rq.outCh), len(rq.outCh))
	is.True(waited < 1000)

	// sending a message will now fail the rq since the
	// outbound channel is full, but with the
	// event fn holding it won't be able to end
	select {
	case rq.bcastCh <- Message{Dest: Tag("foo"), What: what.Inner, Data: ""}:
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
		case <-rq.doneCh:
			is.Fail()
		case <-tmr.C:
			is.Fail()
		default:
			rq.Jaws.Broadcast(Message{Dest: rq})
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
	rq := newTestRequest()
	defer rq.Close()

	rq.Register("SetAttr")
	setAttrElement := rq.GetElements(Tag("SetAttr"))[0]

	rq.Register("RemoveAttr")
	removeAttrElement := rq.GetElements(Tag("RemoveAttr"))[0]

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

	rq.jw.Broadcast(Message{
		Dest: Tag("SetAttr"),
		What: what.SAttr,
		Data: "bar\nbaz",
	})
	rq.jw.Broadcast(Message{
		Dest: Tag("SetAttr"),
		What: what.SAttr,
		Data: "bar\nbaz",
	})
	rq.jw.Broadcast(Message{
		Dest: Tag("RemoveAttr"),
		What: what.RAttr,
		Data: "bar",
	})

	rq.Alert("info", "<html>\nnot\tescaped")
	rq.AlertError(errors.New("<html>\nshould-be-escaped"))
	rq.Redirect("some-url")

	done := false
	for !done {
		select {
		case <-time.NewTimer(testTimeout).C:
			t.Log("timeout")
			t.FailNow()
			done = true
		case msgstr, ok := <-rq.outCh:
			if ok {
				msg, parseok := wsParse([]byte(msgstr))
				if !parseok {
					t.Log(strconv.Quote(msgstr), msg)
					is.Fail()
				}
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
			done = true
		}
	}

	if !strings.HasSuffix(gotSetAttr, "\t\"bar\\nbaz\"\n") {
		t.Log(strconv.Quote(gotSetAttr))
		is.Fail()
	}
	if !(strings.HasPrefix(gotRemoveAttr, "RAttr\t") && strings.HasSuffix(gotRemoveAttr, "\t\"bar\"\n")) {
		t.Log(strconv.Quote(gotRemoveAttr))
		is.Fail()
	}
	is.Equal(gotRedirect, "Redirect\t\t\"some-url\"\n")
	is.Equal(gotInfoAlert, "Alert\t\t\"info\\n<html>\\nnot\\tescaped\"\n")
	is.Equal(gotDangerAlert, "Alert\t\t\"danger\\n&lt;html&gt;\\nshould-be-escaped\"\n")
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
			fmt.Printf("  %d: (%T) id=%q tags=%v\n", i, elem.Ui(), elem.Jid(), elem.Request.TagsOf(elem))
		}
		fmt.Printf("tagMap:\n")
		for tag, elems := range rq.tagMap {
			fmt.Printf(" [%v@%p]: %v\n", tag, tag, elems)
		}
		is.Fail()
	}
}

func jidForTag(rq *Request, tag interface{}) jid.Jid {
	if elems := rq.GetElements(tag); len(elems) > 0 {
		return elems[0].jid
	}
	return 0
}

func TestRequest_Password(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()

	var av atomic.Value
	av.Store("")

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Password(&av, func(rq *Request, jidstr, val string) error {
		defer close(gotCall)
		is.True(rq.GetElement(jid.ParseString(jidstr)) != nil)
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

func TestRequest_Range(t *testing.T) {
	const elemVal = float64(3.14)
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()

	var av atomic.Value
	av.Store(elemVal)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Range(elemId, &av, func(rq *Request, id string, val float64) error {
		defer close(gotCall)
		is.True(rq.GetElement(jid.ParseString(id)) != nil)
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

func TestRequest_Radio(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()

	var av atomic.Value
	av.Store(true)
	elemId := &av

	chk := func(h template.HTML, tag interface{}, txt string) { is.Helper(); checkHtml(is, rq, h, tag, txt) }

	gotCall := make(chan struct{})
	h := rq.Radio(elemId, &av, func(rq *Request, id string, val bool) error {
		defer close(gotCall)
		is.True(rq.GetElement(jid.ParseString(id)) != nil)
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

func TestRequest_ConnectFn(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
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

func TestRequest_WsQueueOverflowCancels(t *testing.T) {
	is := is.New(t)
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
	case <-time.NewTimer(testTimeout).C:
		is.Fail()
	case <-rq.Done():
	}
	is.Equal(context.Cause(rq.Context()), ErrWebsocketQueueOverflow)
}

func TestRequest_Dirty(t *testing.T) {
	is := is.New(t)
	rq := newTestRequest()
	defer rq.Close()

	tss := &testUi{s: "foo"}
	h := rq.UI(NewUiText(tss))
	is.Equal(tss.getCalled, int32(1))
	is.True(strings.Contains(string(h), "foo"))

	rq.Dirty(tss)
	tmr := time.NewTimer(testTimeout)
	for atomic.LoadInt32(&tss.getCalled) < 2 {
		select {
		case <-tmr.C:
			is.Fail()
		default:
			time.Sleep(time.Millisecond)
		}
	}
}
