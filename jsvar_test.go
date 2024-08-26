package jaws

import (
	"html/template"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/linkdata/jaws/what"
)

const varname = "myjsvar"

type valtype struct {
	String string
	Number float64
}

type varmaker struct {
	mu  sync.Mutex
	val string
	err error
}

func (vm *varmaker) JawsVarMake(rq *Request) (UI, error) {
	bind := Bind(&vm.mu, &vm.val)
	return NewJsVar(bind), vm.err
}

type variniter[T comparable] struct {
	JsVar[T]
}

func (vm *variniter[T]) JawsVarInit(rq *Request) error {
	return nil
}

func Test_JsVar_VarIniter(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0
	rq.jw.AddTemplateLookuper(template.Must(template.New("jsvartemplate").Parse(`{{$.JsVar .Dot "` + varname + `"}}`)))

	var mu sync.RWMutex
	var val valtype
	bind := Bind(&mu, &val)
	jsv := NewJsVar(bind)
	dot := &variniter[valtype]{JsVar: jsv}
	elem := rq.NewElement(dot)

	if err := dot.JawsSet(elem, valtype{String: "text", Number: 1.23}); err != nil {
		t.Error(err)
	}

	if err := rq.Template("jsvartemplate", dot); err != nil {
		t.Error(err)
	}

	got := string(rq.BodyHtml())
	want := `<div id="Jid.3" data-jawsdata='{"String":"text","Number":1.23}' data-jawsname="myjsvar" hidden></div>`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q\n", got, want)
	}
}

func Test_JsVar_VarMaker(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0

	dot := &varmaker{
		val: "foo",
	}
	if err := rq.JsVar(dot, varname); err != nil {
		t.Error(err)
	}
	got := string(rq.BodyHtml())
	want := `<div id="Jid.1" data-jawsdata='"foo"' data-jawsname="myjsvar" hidden></div>`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q\n", got, want)
	}

	dot = &varmaker{
		val: "bar",
		err: ErrValueUnchanged,
	}
	if err := rq.JsVar(dot, ""); err != ErrValueUnchanged {
		t.Error(err)
	}
}

type testBind[T comparable] struct {
	Setter[T]
	setCalled chan struct{}
	setCount  int32
}

func (tb *testBind[T]) JawsGet(e *Element) (val T) {
	val = tb.Setter.JawsGet(e)
	return
}

func (tb *testBind[T]) JawsSet(e *Element, val T) (err error) {
	err = tb.Setter.JawsSet(e, val)
	if atomic.AddInt32(&tb.setCount, 1) == 1 {
		close(tb.setCalled)
	}
	return
}

func Test_JsVar_Update(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()
	nextJid = 0

	type valtype struct {
		String string
		Number float64
	}
	var mu sync.Mutex
	var val valtype

	rawbind := Bind(&mu, &val)
	bind := &testBind[valtype]{Setter: rawbind, setCalled: make(chan struct{})}
	rq := newTestRequest()
	defer rq.Close()

	dot := NewJsVar(bind)
	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, []any{varname}); err != nil {
		t.Fatal(err)
	}
	want := `<div id="Jid.1" data-jawsdata='{"String":"","Number":0}' data-jawsname="myjsvar" hidden></div>`
	if sb.String() != want {
		t.Errorf("\n got %q\nwant %q\n", sb.String(), want)
	}
	if err := dot.JawsSet(elem, dot.JawsGet(elem)); err != ErrValueUnchanged {
		t.Error(err)
	}
	if err := dot.JawsSet(elem, valtype{"x", 2}); err != nil {
		t.Error(err)
	}
	rq.Dirty(dot.Setter)

	select {
	case <-th.C:
		th.Timeout()
	case gotMsg := <-rq.outCh:
		wantMsg := wsMsg{
			Data: `{"String":"x","Number":2}`,
			Jid:  1,
			What: what.Set,
		}
		if !reflect.DeepEqual(gotMsg, wantMsg) {
			t.Errorf("\n got %v\nwant %v\n", gotMsg, wantMsg)
		}
	}
}

func Test_JsVar_Event(t *testing.T) {
	th := newTestHelper(t)
	jw := New()
	defer jw.Close()
	nextJid = 0

	const varname = "myjsvar"
	type valtype struct {
		String string
		Number float64
	}
	var mu sync.Mutex
	var val valtype

	rawbind := Bind(&mu, &val)
	bind := &testBind[valtype]{Setter: rawbind, setCalled: make(chan struct{})}
	rq := newTestRequest()
	defer rq.Close()

	dot := NewJsVar(bind)
	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, []any{varname}); err != nil {
		t.Fatal(err)
	}
	want := `<div id="Jid.1" data-jawsdata='{"String":"","Number":0}' data-jawsname="myjsvar" hidden></div>`
	if sb.String() != want {
		t.Errorf("\n got %q\nwant %q\n", sb.String(), want)
	}

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: `{"String":"y","Number":3}`}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-bind.setCalled:
	}

	th.Equal(val, valtype{"y", 3})
}

func Test_JsVar_PanicsOnWrongType(t *testing.T) {
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	defer func() {
		if x := recover(); x == nil {
			th.Fail()
		}
	}()
	rq.JsVar(1, "")
	th.Fail()
}
