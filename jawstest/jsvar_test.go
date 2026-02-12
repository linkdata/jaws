package jawstest

import (
	"html/template"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/core"
	"github.com/linkdata/jaws/what"
)

type jsVarValue struct {
	String string
	Number float64
}

func waitMsg(t *testing.T, ch <-chan core.WsMsg) core.WsMsg {
	t.Helper()
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for ws message")
		return core.WsMsg{}
	case msg := <-ch:
		return msg
	}
}

func TestJsVar_RenderThroughTemplate(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	rq.Jaws.AddTemplateLookuper(template.Must(template.New("jsvartemplate").Parse(`{{$.JsVar "myjsvar" .Dot}}`)))

	var mu sync.Mutex
	val := jsVarValue{String: "text", Number: 1.23}
	jsv := jaws.NewJsVar(&mu, &val)

	if err := rq.Template("jsvartemplate", jsv); err != nil {
		t.Fatal(err)
	}
	got := rq.BodyString()
	if !strings.Contains(got, `data-jawsname="myjsvar"`) {
		t.Fatalf("missing jaws name: %q", got)
	}
	if !strings.Contains(got, `"String":"text"`) {
		t.Fatalf("missing serialized value: %q", got)
	}
}

type testJsVarMaker struct{}

func (t *testJsVarMaker) JawsMakeJsVar(*jaws.Request) (jaws.IsJsVar, error) {
	var mu sync.Mutex
	v := "quote(')"
	return jaws.NewJsVar(&mu, &v), nil
}

func TestJsVar_JsVarMaker(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	if err := rq.JsVar("foo", &testJsVarMaker{}); err != nil {
		t.Fatal(err)
	}
	if got := rq.BodyString(); !strings.Contains(got, `data-jawsname="foo"`) {
		t.Fatalf("unexpected body: %q", got)
	}
}

func TestJsVar_EventRoundtrip(t *testing.T) {
	rq := newTestRequest(t)
	defer rq.Close()

	var mu sync.Mutex
	var val jsVarValue
	jsv := jaws.NewJsVar(&mu, &val)

	elem := rq.NewElement(jsv)
	var sb strings.Builder
	if err := jsv.JawsRender(elem, &sb, []any{"myjsvar"}); err != nil {
		t.Fatal(err)
	}

	rq.InCh <- core.WsMsg{Jid: elem.Jid(), What: what.Set, Data: `={"String":"y","Number":3}`}
	msg := waitMsg(t, rq.OutCh)

	if msg.What != what.Set {
		t.Fatalf("unexpected what: %v", msg.What)
	}
	if msg.Jid != elem.Jid() {
		t.Fatalf("unexpected jid: got %v want %v", msg.Jid, elem.Jid())
	}
	if !strings.Contains(msg.Data, `"String":"y"`) {
		t.Fatalf("unexpected data: %q", msg.Data)
	}
	if val.String != "y" || val.Number != 3 {
		t.Fatalf("unexpected value: %#v", val)
	}
}
