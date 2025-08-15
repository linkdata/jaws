package jaws

import (
	"encoding/json"
	"html/template"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/linkdata/jaws/what"
)

const varname = "myjsvar"

type valtype struct {
	String string
	Number float64
}

type variniter[T comparable] struct {
	JsVar[T]
}

var (
	_ isJsVar = &variniter[int]{}
)

func Test_JsVar_JawsRender(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0
	rq.jw.AddTemplateLookuper(template.Must(template.New("jsvartemplate").Parse(`{{$.JsVar "` + varname + `" .Dot}}`)))

	var mu sync.RWMutex
	var val valtype
	jsv := NewJsVar(&mu, &val)
	dot := jsv
	elem := rq.NewElement(dot)

	if err := dot.JawsSet(elem, valtype{String: "text", Number: 1.23}); err != nil {
		t.Error(err)
	}

	if val.String != "text" {
		t.Error(val)
	}

	x := dot.JawsGet(elem)
	if !reflect.DeepEqual(x, val) {
		t.Error(x)
	}

	if err := rq.Template("jsvartemplate", dot); err != nil {
		t.Error(err)
	}

	got := string(rq.BodyHTML())
	want := `<div id="Jid.3" data-jawsdata='{"String":"text","Number":1.23}' data-jawsname="myjsvar" hidden></div>`
	if got != want {
		t.Errorf("\n got: %q\nwant: %q\n", got, want)
	}
}

func Test_JsVar_Update(t *testing.T) {
	th := newTestHelper(t)
	jw, _ := New()
	defer jw.Close()
	nextJid = 0

	type valtype struct {
		String string
		Number float64
	}
	var mu sync.Mutex
	var val valtype
	dot := NewJsVar(&mu, &val)

	rq := newTestRequest()
	defer rq.Close()

	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, []any{varname}); err != nil {
		t.Fatal(err)
	}
	want := `<div id="Jid.1" data-jawsdata='{"String":"","Number":0}' data-jawsname="myjsvar" hidden></div>`
	if sb.String() != want {
		t.Errorf("\n got %q\nwant %q\n", sb.String(), want)
	}
	if err := dot.JawsSet(elem, valtype{"x", 2}); err != nil {
		t.Error(err)
	}
	rq.Dirty(dot)

	select {
	case <-th.C:
		th.Timeout()
	case gotMsg := <-rq.outCh:
		wantMsg := wsMsg{
			Data: "\t{\"String\":\"x\",\"Number\":2}",
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
	jw, _ := New()
	defer jw.Close()
	nextJid = 0

	const varname = "myjsvar"
	type valtype struct {
		String string
		Number float64
	}
	var mu sync.Mutex
	var val valtype
	tl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	dot := NewJsVar(&tl, &val)

	rq := newTestRequest()
	defer rq.Close()

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
	case <-tl.unlockCalled:
		tl.reset()
	}

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "\t{\"String\":\"y\",\"Number\":3}"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case <-tl.unlockCalled:
	}

	th.Equal(val, valtype{"y", 3})

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		after, found := strings.CutPrefix(s, "Set\tJid.1\t\t")
		th.Equal(found, true)
		if found {
			var x valtype
			err := json.Unmarshal([]byte(after), &x)
			th.NoErr(err)
			th.Equal(x, valtype{"y", 3})
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case rq.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "\t1"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq.outCh:
		s := msg.Format()
		if !strings.Contains(s, "jq: expected jaws.valtype, not float64") {
			th.Error(s)
		}
	}
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
	rq.JsVar("", 1)
	th.Fail()
}

func Test_JsVar_AppendJSON_PanicsOnFailure(t *testing.T) {
	defer func() {
		if x := recover(); x == nil {
			t.Fail()
		}
	}()
	var mu sync.Mutex
	ch := make(chan int)

	jsv := NewJsVar(&mu, &ch)
	jsv.AppendJSON(nil, nil)
	t.Fail()
}
