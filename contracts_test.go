package jaws

import (
	"html/template"
	"testing"

	"github.com/linkdata/jaws/lib/what"
	"github.com/linkdata/jaws/lib/wire"
)

type testJawsClick struct {
	clickCh chan string
	*testSetter[string]
}

func (tjc *testJawsClick) JawsClick(e *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click.Name
	}
	return
}

var _ ClickHandler = (*testJawsClick)(nil)

type testJawsContextMenu struct {
	clickCh chan Click
	*testSetter[Click]
}

func (tjc *testJawsContextMenu) JawsContextMenu(e *Element, click Click) (err error) {
	if err = tjc.Err(); err == nil {
		tjc.clickCh <- click
	}
	return
}

var _ ContextMenuHandler = (*testJawsContextMenu)(nil)

func Test_clickHandlerWapper_JawsEvent(t *testing.T) {
	th := newTestHelper(t)
	NextJid = 0
	rq := newTestRequest(t)
	defer rq.Close()

	tjc := &testJawsClick{
		clickCh:    make(chan string),
		testSetter: newTestSetter(""),
	}

	want := `<div id="Jid.1">inner</div>`
	rq.UI(testDivWidget{inner: template.HTML("inner")}, tjc)
	if got := rq.BodyString(); got != want {
		t.Errorf("Request.UI(NewDiv()) = %q, want %q", got, want)
	}

	rq.InCh <- wire.WsMsg{Data: "text", Jid: 1, What: what.Input}
	select {
	case <-th.C:
		th.Timeout()
	case s := <-rq.OutCh:
		t.Errorf("%q", s)
	default:
	}

	rq.InCh <- wire.WsMsg{Data: "adam", Jid: 1, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case name := <-tjc.clickCh:
		t.Fatalf("malformed click should be ignored, got %q", name)
	default:
	}

	rq.InCh <- wire.WsMsg{Data: "adam\t1\t2", Jid: 1, What: what.Click}
	select {
	case <-th.C:
		th.Timeout()
	case name := <-tjc.clickCh:
		if name != "adam" {
			t.Error(name)
		}
	}
}

func Test_defaultAuth(t *testing.T) {
	a := DefaultAuth{}
	if a.Data() != nil {
		t.Fatal()
	}
	if a.Email() != "" {
		t.Fatal()
	}
	if a.IsAdmin() != true {
		t.Fatal()
	}
}
