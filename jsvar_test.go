package jaws

import (
	"encoding/json"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"testing"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

const varname = "myjsvar"

type valtype struct {
	String string
	Number float64
}

func Test_JsVar_JawsRender(t *testing.T) {
	rq := newTestRequest()
	defer rq.Close()

	nextJid = 0
	rq.jw.AddTemplateLookuper(template.Must(template.New("jsvartemplate").Parse(`{{$.JsVar "` + varname + `" .Dot}}`)))

	var mu deadlock.RWMutex
	var val valtype
	jsv := NewJsVar(&mu, &val)
	dot := jsv
	elem := rq.NewElement(dot)

	jsv.JawsUpdate(nil) // no-op, just to satisfy coverage

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
	want := `<div id="Jid.3" data-jawsname="myjsvar" data-jawsdata='{"String":"text","Number":1.23}' hidden></div>`
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
	var mu deadlock.Mutex
	var val valtype
	dot := NewJsVar(&mu, &val)

	rq := newTestRequest()
	defer rq.Close()

	elem := rq.NewElement(dot)
	var sb strings.Builder
	if err := dot.JawsRender(elem, &sb, []any{varname}); err != nil {
		t.Fatal(err)
	}
	want := `<div id="Jid.1" data-jawsname="myjsvar" data-jawsdata='{"String":"","Number":0}' hidden></div>`
	if sb.String() != want {
		t.Errorf("\n got %q\nwant %q\n", sb.String(), want)
	}
	if err := dot.JawsSet(elem, valtype{"x", 2}); err != nil {
		t.Error(err)
	}
	// rq.Dirty(dot)

	select {
	case <-th.C:
		th.Timeout()
	case gotMsg := <-rq.outCh:
		wantMsg := wsMsg{
			Data: "={\"String\":\"x\",\"Number\":2}",
			Jid:  1,
			What: what.Set,
		}
		if !reflect.DeepEqual(gotMsg, wantMsg) {
			t.Errorf("\n got %v\nwant %v\n", gotMsg, wantMsg)
		}
	}

	elem.JsSet("", "") // just to satisfy coverage
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
	var mu deadlock.Mutex
	var val valtype
	tl := testLocker{Locker: &mu, unlockCalled: make(chan struct{})}
	dot := NewJsVar(&tl, &val)

	tj := newTestJaws()
	defer tj.Close()
	const expectedHTML = `<div id="Jid.%d" data-jawsname="myjsvar" data-jawsdata='{"String":"","Number":0}' hidden></div>`

	rq1 := tj.newRequest(nil)
	elem1 := rq1.NewElement(dot)
	var sb1 strings.Builder
	if err := dot.JawsRender(elem1, &sb1, []any{varname}); err != nil {
		t.Fatal(err)
	}
	th.Equal(sb1.String(), fmt.Sprintf(expectedHTML, 1))

	rq2 := tj.newRequest(nil)
	elem2 := rq2.NewElement(dot)
	var sb2 strings.Builder
	if err := dot.JawsRender(elem2, &sb2, []any{varname}); err != nil {
		t.Fatal(err)
	}
	th.Equal(sb2.String(), fmt.Sprintf(expectedHTML, 2))

	select {
	case <-th.C:
		th.Timeout()
	case <-tl.unlockCalled:
	}

	tl.reset()

	select {
	case <-th.C:
		th.Timeout()
	case rq1.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "={\"String\":\"y\",\"Number\":3}"}:
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
	case msg := <-rq1.outCh:
		s := msg.Format()
		after, found := strings.CutPrefix(s, "Set\tJid.1\t=")
		th.Equal(found, true)
		if found {
			var x valtype
			err := json.Unmarshal([]byte(after), &x)
			th.NoErr(err)
			th.Equal(x, valtype{"y", 3})
		} else {
			t.Fatalf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq2.outCh:
		s := msg.Format()
		after, found := strings.CutPrefix(s, "Set\tJid.2\t=")
		th.Equal(found, true)
		if found {
			var x valtype
			err := json.Unmarshal([]byte(after), &x)
			th.NoErr(err)
			th.Equal(x, valtype{"y", 3})
		} else {
			t.Fatalf("%q", s)
		}
	}

	select {
	case <-th.C:
		th.Timeout()
	case rq1.inCh <- wsMsg{Jid: 1, What: what.Set, Data: "=1"}:
	}

	select {
	case <-th.C:
		th.Timeout()
	case msg := <-rq1.outCh:
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

type testJsVarMaker struct {
}

func (t *testJsVarMaker) JawsMakeJsVar(rq *Request) (v IsJsVar, err error) {
	var mu deadlock.Mutex
	val := "quote(')"
	return NewJsVar(&mu, &val), nil
}

var _ JsVarMaker = &testJsVarMaker{}

func Test_JsVar_JsVarMaker(t *testing.T) {
	nextJid = 0
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()
	err := rq.JsVar("foo", &testJsVarMaker{})
	th.NoErr(err)
	th.Equal(rq.BodyHTML(), template.HTML("<div id=\"Jid.1\" data-jawsname=\"foo\" data-jawsdata='\"quote(\\u0027)\"' hidden></div>"))
}

type testJsVarPathSetter struct {
	Value string
}

func (t *testJsVarPathSetter) JawsSetPath(elem *Element, jspath string, value any) (changed bool, err error) {
	t.Value = value.(string) + "!!"
	return true, nil
}

var _ PathSetter = &testJsVarPathSetter{}

func Test_JsVar_PathSetter(t *testing.T) {
	nextJid = 0
	th := newTestHelper(t)
	rq := newTestRequest()
	defer rq.Close()

	var mu deadlock.Mutex
	var val testJsVarPathSetter
	jsv := NewJsVar(&mu, &val)
	elem := rq.NewElement(jsv)
	_, err := jsv.JawsSetPath(elem, "", "foo")
	th.NoErr(err)
	th.Equal(val.Value, "foo!!")
}
