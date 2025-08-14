package jaws

import (
	"html/template"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/linkdata/jaws/what"
)

type testDotStruct struct {
	Arg  IsJsVar
	Retv IsJsVar
}

func TestJsFunc_JawsRender(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0
	rq.jw.AddTemplateLookuper(template.Must(template.New("jsfunctemplate").Parse(`{{$.JsFunc "somefn" .Dot.Arg .Dot.Retv "someattr"}}`)))

	var mu sync.RWMutex
	var argval float64
	arg := NewJsVar(&argval, &mu)

	var retvval string
	retv := NewJsVar(&retvval, &mu)

	dot := testDotStruct{
		Arg:  arg,
		Retv: retv,
	}

	elem := rq.NewElement(arg)

	if err := rq.Template("jsfunctemplate", dot); err != nil {
		t.Error(err)
	}

	got := string(rq.BodyHTML())
	want := `<div id="Jid.3" data-jawsname="somefn" someattr hidden></div>`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q\n", got, want)
	}

	arg.JawsSet(elem, 1.3)
	rq.Dirty(arg)

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		got := msg.Format()
		want := "Call\tJid.3\t1.3\n"
		if got != want {
			t.Error(strconv.Quote(got))
		}
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

	var mu sync.RWMutex
	argtl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	var argval float64
	arg := NewJsVar(&argval, &argtl)

	var retvval string
	retvtl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	retv := NewJsVar(&retvval, &retvtl)

	dot := NewJsFunc(arg, retv)
	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, []any{"fnname"}); err != nil {
		t.Fatal(err)
	}
	wantHTML := "<div id=\"Jid.1\" data-jawsname=\"fnname\" hidden></div>"
	if gotHTML := sb.String(); gotHTML != wantHTML {
		t.Errorf("\n got %q\nwant %q\n", gotHTML, wantHTML)
	}

	th.Equal(retvval, "")

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "\t\"sometext\""}:
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
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "\t1.2"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if !strings.Contains(s, "jq: expected string, not float64") {
			th.Error(s)
		}
	}

	/*vm := &varmaker{
		val: "bar",
		err: ErrValueUnchanged,
	}
	if err := rq.JsFunc("", arg, vm); err != ErrValueUnchanged {
		t.Error(err)
	}*/
}
