package jaws

import (
	"html/template"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestJsFunc_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	rq.jw.AddTemplateLookuper(template.Must(template.New("jsfunctemplate").Parse(`{{$.JsFunc .Dot "someattr"}}{{$.JsFunc "somefn2"}}`)))

	dot := NewJsFunc("somefn")
	elem := rq.NewElement(dot)
	th.Equal(elem.Jid(), Jid(1))

	if err := rq.Template("jsfunctemplate", dot); err != nil {
		t.Error(err)
	}

	th.Equal(string(rq.BodyHTML()), `<div id="Jid.3" data-jawsname="somefn" someattr hidden></div>`+
		"\n"+`<div id="Jid.4" data-jawsname="somefn2" hidden></div>`)
}

func TestJsFunc_JsCall(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	dot := NewJsFunc("somefn")

	dot.IsJsFunc() // no-op

	var sb strings.Builder
	elem := rq.NewElement(dot)
	th.Equal(elem.Jid(), Jid(1))
	err := elem.JawsRender(&sb, nil)
	th.NoErr(err)

	dot.JawsUpdate(elem) // no-op

	elem.Jaws.JsCall(dot, "1.3")

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		got := msg.Format()
		th.Equal(got, "Call\tJid.1\t1.3\n")
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

	dot := NewJsFunc("fnname")
	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, nil); err != nil {
		t.Fatal(err)
	}
	wantHTML := "\n<div id=\"Jid.1\" data-jawsname=\"fnname\" hidden></div>"
	th.Equal(sb.String(), wantHTML)
}
