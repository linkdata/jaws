package jaws

import (
	"html/template"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws/what"
)

type DotStruct struct {
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
	argbind := Bind(&mu, &argval)
	arg := NewJsVar(argbind)

	var retvval string
	retvbind := Bind(&mu, &retvval)
	retv := NewJsVar(retvbind)

	dot := DotStruct{
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

func TestJsFunc_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	nextJid = 0
	rq := newTestRequest()
	defer rq.Close()

	var mu sync.RWMutex
	var argval float64
	argbind := Bind(&mu, &argval)
	arg := NewJsVar(argbind)

	var retvval string
	rawbind := Bind(&mu, &retvval)
	retvbind := &testBind[string]{Setter: rawbind, setCalled: make(chan struct{})}
	retv := NewJsVar(retvbind)

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
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: `"sometext"`}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-retvbind.setCalled:
	}

	th.Equal(argval, float64(0))
	th.Equal(retvval, "sometext")

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: `1.2`}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if !strings.Contains(s, "cannot unmarshal") {
			th.Error(s)
		}
	}

	vm := &varmaker{
		val: "bar",
		err: ErrValueUnchanged,
	}
	if err := rq.JsFunc("", arg, vm); err != ErrValueUnchanged {
		t.Error(err)
	}
}
