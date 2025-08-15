package jaws

import (
	"html/template"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

func TestJsFunc_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0
	rq.jw.AddTemplateLookuper(template.Must(template.New("jsfunctemplate").Parse(`{{$.JsFunc .Dot "someattr"}}`)))

	var mu deadlock.RWMutex
	var argval float64
	arg := Bind(&mu, &argval)

	var retvval string
	retv := Bind(&mu, &retvval)
	dot := NewJsFunc("somefn", arg, retv)

	elem := rq.NewElement(dot)

	if err := rq.Template("jsfunctemplate", dot); err != nil {
		t.Error(err)
	}

	got := string(rq.BodyHTML())
	th.Equal(got, "\n"+`<div id="Jid.3" data-jawsname="somefn" someattr hidden></div>`+"\n")

	arg.JawsSet(elem, 1.3)
	rq.Dirty(arg)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		got := msg.Format()
		th.Equal(got, "Call\tJid.3\t1.3\n")
	}
}

type testLocker struct {
	sync.Locker
	unlockCalled chan struct{}
	unlockCount  int32
}

func (tl *testLocker) reset() {
	tl.unlockCalled = make(chan struct{})
	atomic.StoreInt32(&tl.unlockCount, 0)
}

func (tl *testLocker) Unlock() {
	tl.Locker.Unlock()
	if atomic.AddInt32(&tl.unlockCount, 1) == 1 {
		if tl.unlockCalled != nil {
			close(tl.unlockCalled)
		}
	}
}

func TestJsFunc_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	var mu deadlock.RWMutex
	var argval float64
	argtl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	arg := Bind(&argtl, &argval)

	var retvval string
	retvtl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	retv := Bind(&retvtl, &retvval)

	dot := NewJsFunc("fnname", arg, retv)
	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, nil); err != nil {
		t.Fatal(err)
	}
	wantHTML := "\n<div id=\"Jid.1\" data-jawsname=\"fnname\" hidden></div>\n"
	th.Equal(sb.String(), wantHTML)

	th.Equal(retvval, "")

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "=\"sometext\""}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-retvtl.unlockCalled:
	}

	th.Equal(argval, float64(0))
	th.Equal(retvval, "sometext")

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "=1.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if !strings.Contains(s, "cannot unmarshal number") {
			th.Error(s)
		}
	}
}
